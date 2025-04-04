package starlark

import (
	"encoding/base64"
	"errors"
	"fmt"
	"hash/fnv"
	go_path "path"

	pg_label "github.com/buildbarn/bonanza/pkg/label"
	model_core "github.com/buildbarn/bonanza/pkg/model/core"
	model_starlark_pb "github.com/buildbarn/bonanza/pkg/proto/model/starlark"
	"github.com/buildbarn/bonanza/pkg/storage/object"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

const externalDirectoryName = "external"

type File[TReference object.BasicReference, TMetadata model_core.CloneableReferenceMetadata] struct {
	definition model_core.Message[*model_starlark_pb.File, TReference]
}

var (
	_ EncodableValue[object.LocalReference, model_core.CloneableReferenceMetadata] = (*File[object.LocalReference, model_core.CloneableReferenceMetadata])(nil)
	_ starlark.Comparable                                                          = (*File[object.LocalReference, model_core.CloneableReferenceMetadata])(nil)
	_ starlark.HasAttrs                                                            = (*File[object.LocalReference, model_core.CloneableReferenceMetadata])(nil)
)

func NewFile[TReference object.BasicReference, TMetadata model_core.CloneableReferenceMetadata](definition model_core.Message[*model_starlark_pb.File, TReference]) *File[TReference, TMetadata] {
	return &File[TReference, TMetadata]{
		definition: definition,
	}
}

func (f *File[TReference, TMetadata]) String() string {
	if p, err := f.getPath(); err == nil {
		return fmt.Sprintf("<File %s>", p)
	}
	return "<File>"
}

func (File[TReference, TMetadata]) Type() string {
	return "File"
}

func (File[TReference, TMetadata]) Freeze() {
}

func (File[TReference, TMetadata]) Truth() starlark.Bool {
	return starlark.True
}

func (f *File[TReference, TMetadata]) Hash(thread *starlark.Thread) (uint32, error) {
	d := f.definition.Message
	h := fnv.New32a()
	h.Write([]byte(d.Package))
	h.Write([]byte(":"))
	h.Write([]byte(d.PackageRelativePath))
	return h.Sum32(), nil
}

func (f *File[TReference, TMetadata]) equals(other *File[TReference, TMetadata]) bool {
	if f != other {
		if !model_core.MessagesEqual(f.definition, other.definition) {
			return false
		}
	}
	return true
}

func (f *File[TReference, TMetadata]) CompareSameType(thread *starlark.Thread, op syntax.Token, other starlark.Value, depth int) (bool, error) {
	switch op {
	case syntax.EQL:
		return f.equals(other.(*File[TReference, TMetadata])), nil
	case syntax.NEQ:
		return !f.equals(other.(*File[TReference, TMetadata])), nil
	default:
		return false, errors.New("File can only be compared for equality")
	}
}

func (f *File[TReference, TMetadata]) appendOwner(parts []string) ([]string, error) {
	if o := f.definition.Message.Owner; o != nil {
		// Place output files in a directory that has the
		// configuration reference in its name. In addition to
		// ensuring that output files for different
		// configurations don't collide, it makes it easy to
		// inspect the configuration that was used to build.
		configurationSlug := "_"
		if o.ConfigurationReference != nil {
			configurationReference, err := model_core.FlattenReference(
				model_core.Nested(f.definition, o.ConfigurationReference),
			)
			if err != nil {
				return nil, err
			}
			configurationSlug = base64.RawURLEncoding.EncodeToString(configurationReference.GetRawReference())
		}

		parts = append(
			parts,
			"bazel-out",
			configurationSlug,
			"bin",
		)
	}
	return parts, nil
}

func (f *File[TReference, TMetadata]) getPath() (string, error) {
	d := f.definition.Message
	canonicalPackage, err := pg_label.NewCanonicalPackage(d.Package)
	if err != nil {
		return "", fmt.Errorf("invalid canonical package %#v: %w", d.Package, err)
	}
	parts, err := f.appendOwner(make([]string, 0, 7))
	if err != nil {
		return "", err
	}
	return go_path.Join(
		append(
			parts,
			externalDirectoryName,
			canonicalPackage.GetCanonicalRepo().String(),
			canonicalPackage.GetPackagePath(),
			d.PackageRelativePath,
		)...,
	), nil
}

func (f *File[TReference, TMetadata]) Attr(thread *starlark.Thread, name string) (starlark.Value, error) {
	d := f.definition.Message
	switch name {
	case "basename":
		return starlark.String(go_path.Base(d.PackageRelativePath)), nil
	case "dirname":
		p, err := f.getPath()
		if err != nil {
			return nil, err
		}
		return starlark.String(go_path.Dir(p)), nil
	case "extension":
		p := d.PackageRelativePath
		for i := len(p) - 1; i >= 0 && p[i] != '/'; i-- {
			if p[i] == '.' {
				return starlark.String(p[i+1:]), nil
			}
		}
		return starlark.String(""), nil
	case "is_directory":
		return starlark.Bool(d.Type == model_starlark_pb.File_DIRECTORY), nil
	case "is_source":
		return starlark.Bool(d.Owner == nil), nil
	case "is_symlink":
		return starlark.Bool(d.Type == model_starlark_pb.File_SYMLINK), nil
	case "owner":
		canonicalPackage, err := pg_label.NewCanonicalPackage(d.Package)
		if err != nil {
			return nil, fmt.Errorf("invalid canonical package %#v: %w", d.Package, err)
		}

		// If the file is an output file, return the label of
		// the target that generates it. If it is a source file,
		// return a label of the file itself.
		targetNameStr := d.PackageRelativePath
		if o := d.Owner; o != nil {
			targetNameStr = o.TargetName
		}
		targetName, err := pg_label.NewTargetName(targetNameStr)
		if err != nil {
			return nil, fmt.Errorf("invalid target name %#v: %w", targetNameStr, err)
		}

		return NewLabel[TReference, TMetadata](canonicalPackage.AppendTargetName(targetName).AsResolved()), nil
	case "path":
		p, err := f.getPath()
		if err != nil {
			return nil, err
		}
		return starlark.String(p), nil
	case "root":
		canonicalPackage, err := pg_label.NewCanonicalPackage(d.Package)
		if err != nil {
			return nil, fmt.Errorf("invalid canonical package %#v: %w", d.Package, err)
		}
		parts, err := f.appendOwner(make([]string, 0, 6))
		if err != nil {
			return nil, err
		}
		return newStructFromLists[TReference, TMetadata](
			nil,
			[]string{"path"},
			[]any{
				starlark.String(go_path.Join(
					append(
						parts,
						externalDirectoryName,
						canonicalPackage.GetCanonicalRepo().String(),
					)...,
				)),
			},
		), nil
	case "short_path":
		canonicalPackage, err := pg_label.NewCanonicalPackage(d.Package)
		if err != nil {
			return nil, fmt.Errorf("invalid canonical package %#v: %w", d.Package, err)
		}
		return starlark.String(go_path.Join(
			"..",
			canonicalPackage.GetCanonicalRepo().String(),
			canonicalPackage.GetPackagePath(),
			d.PackageRelativePath,
		)), nil
	default:
		return nil, nil
	}
}

var fileAttrNames = []string{
	"basename",
	"dirname",
	"extension",
	"is_directory",
	"is_source",
	"is_symlink",
	"owner",
	"path",
	"root",
	"short_path",
}

func (File[TReference, TMetadata]) AttrNames() []string {
	return fileAttrNames
}

func (f *File[TReference, TMetadata]) EncodeValue(path map[starlark.Value]struct{}, currentIdentifier *pg_label.CanonicalStarlarkIdentifier, options *ValueEncodingOptions[TReference, TMetadata]) (model_core.PatchedMessage[*model_starlark_pb.Value, TMetadata], bool, error) {
	d := model_core.Patch(options.ObjectCapturer, f.definition)
	return model_core.NewPatchedMessage(
		&model_starlark_pb.Value{
			Kind: &model_starlark_pb.Value_File{
				File: d.Message,
			},
		},
		d.Patcher,
	), false, nil
}
