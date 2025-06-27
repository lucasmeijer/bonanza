package parser

import (
	model_core "bonanza.build/pkg/model/core"
)

type ObjectParser[TReference, TParsedObject any] interface {
	ParseObject(in model_core.Message[[]byte, TReference], decodingParameters []byte) (TParsedObject, int, error)
	GetDecodingParametersSizeBytes() int
}
