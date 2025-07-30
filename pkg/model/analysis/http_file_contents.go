package analysis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"math"
	"net/http"

	"bonanza.build/pkg/evaluation"
	model_core "bonanza.build/pkg/model/core"
	model_filesystem "bonanza.build/pkg/model/filesystem"
	model_analysis_pb "bonanza.build/pkg/proto/model/analysis"
	model_fetch_pb "bonanza.build/pkg/proto/model/fetch"
	"bonanza.build/pkg/storage/dag"

	"github.com/buildbarn/bb-storage/pkg/filesystem"
	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
)

func (c *baseComputer[TReference, TMetadata]) ComputeHttpFileContentsValue(ctx context.Context, key *model_analysis_pb.HttpFileContents_Key, e HttpFileContentsEnvironment[TReference, TMetadata]) (PatchedHttpFileContentsValue, error) {
	fileCreationParameters, gotFileCreationParameters := e.GetFileCreationParametersObjectValue(&model_analysis_pb.FileCreationParametersObject_Key{})
	if !gotFileCreationParameters {
		return PatchedHttpFileContentsValue{}, evaluation.ErrMissingDependency
	}

ProcessURLs:
	for _, url := range key.FetchOptions.GetTarget().GetUrls() {
		// Store copies of the file in a local cache directory.
		// TODO: Remove this feature once our storage is robust enough.
		urlHash := sha256.Sum256([]byte(url))
		filename := path.MustNewComponent(hex.EncodeToString(urlHash[:]))
		downloadedFile, err := c.cacheDirectory.OpenReadWrite(filename, filesystem.DontCreate)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return PatchedHttpFileContentsValue{}, err
			}
			downloadedFile, err = c.cacheDirectory.OpenReadWrite(filename, filesystem.CreateExcl(0o666))
			if err != nil {
				return PatchedHttpFileContentsValue{}, err
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				downloadedFile.Close()
				return PatchedHttpFileContentsValue{}, err
			}
			for _, entry := range key.FetchOptions.GetTarget().GetHeaders() {
				req.Header.Set(entry.Name, entry.Value)
			}

			resp, err := c.httpClient.Do(req)
			if err != nil {
				downloadedFile.Close()
				return PatchedHttpFileContentsValue{}, err
			}
			defer resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusOK:
				// Download the file to the local system.
				if _, err := io.Copy(model_filesystem.NewSectionWriter(downloadedFile), resp.Body); err != nil {
					downloadedFile.Close()
					c.cacheDirectory.Remove(filename)
					return PatchedHttpFileContentsValue{}, err
				}
			case http.StatusNotFound:
				downloadedFile.Close()
				c.cacheDirectory.Remove(filename)
				continue ProcessURLs
			default:
				downloadedFile.Close()
				c.cacheDirectory.Remove(filename)
				if key.FetchOptions.GetAllowFail() {
					continue ProcessURLs
				}
				return PatchedHttpFileContentsValue{}, fmt.Errorf("received unexpected HTTP response %#v", resp.Status)
			}
		}

		var hasher hash.Hash
		if integrity := key.FetchOptions.GetTarget().GetIntegrity(); integrity == nil {
			hasher = sha256.New()
		} else {
			switch integrity.HashAlgorithm {
			case model_fetch_pb.SubresourceIntegrity_SHA256:
				hasher = sha256.New()
			case model_fetch_pb.SubresourceIntegrity_SHA384:
				hasher = sha512.New384()
			case model_fetch_pb.SubresourceIntegrity_SHA512:
				hasher = sha512.New()
			default:
				downloadedFile.Close()
				c.cacheDirectory.Remove(filename)
				return PatchedHttpFileContentsValue{}, errors.New("unknown subresource integrity hash algorithm")
			}
		}
		if _, err := io.Copy(hasher, io.NewSectionReader(downloadedFile, 0, math.MaxInt64)); err != nil {
			downloadedFile.Close()
			c.cacheDirectory.Remove(filename)
			return PatchedHttpFileContentsValue{}, fmt.Errorf("failed to hash file: %w", err)
		}
		hash := hasher.Sum(nil)

		var sha256 []byte
		if integrity := key.FetchOptions.GetTarget().GetIntegrity(); integrity == nil {
			sha256 = hash
		} else if !bytes.Equal(hash, integrity.Hash) {
			downloadedFile.Close()
			c.cacheDirectory.Remove(filename)
			return PatchedHttpFileContentsValue{}, fmt.Errorf("file has hash %s, while %s was expected", hex.EncodeToString(hash), hex.EncodeToString(integrity.Hash))
		}

		// Compute a Merkle tree of the file and return it. The
		// downloaded file is removed after uploading completes.
		// any chunks of data in memory, as we would consume a large
		// amount of memory otherwise.
		fileMerkleTree, err := model_filesystem.CreateChunkDiscardingFileMerkleTree(ctx, fileCreationParameters, downloadedFile)
		if err != nil {
			return PatchedHttpFileContentsValue{}, err
		}
		if fileMerkleTree.Message == nil {
			return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.HttpFileContents_Value{
				Exists: &model_analysis_pb.HttpFileContents_Value_Exists{},
			}), nil
		}
		return model_core.NewPatchedMessage(
			&model_analysis_pb.HttpFileContents_Value{
				Exists: &model_analysis_pb.HttpFileContents_Value_Exists{
					Contents: fileMerkleTree.Message,
					Sha256:   sha256,
				},
			},
			fileMerkleTree.Patcher,
		), nil
	}

	// File not found.
	return model_core.NewSimplePatchedMessage[dag.ObjectContentsWalker](&model_analysis_pb.HttpFileContents_Value{}), nil
}
