package parser

import (
	model_core "bonanza.build/pkg/model/core"
)

type chainedObjectParser[TReference, TParsedObject any] struct {
	parserA ObjectParser[TReference, model_core.Message[[]byte, TReference]]
	parserB ObjectParser[TReference, TParsedObject]
}

func NewChainedObjectParser[TReference, TParsedObject any](parserA ObjectParser[TReference, model_core.Message[[]byte, TReference]], parserB ObjectParser[TReference, TParsedObject]) ObjectParser[TReference, TParsedObject] {
	return &chainedObjectParser[TReference, TParsedObject]{
		parserA: parserA,
		parserB: parserB,
	}
}

func (p *chainedObjectParser[TReference, TParsedObject]) ParseObject(in model_core.Message[[]byte, TReference], decodingParameters []byte) (TParsedObject, int, error) {
	v, _, err := p.parserA.ParseObject(in, decodingParameters)
	if err != nil {
		var bad TParsedObject
		return bad, 0, err
	}
	return p.parserB.ParseObject(v, nil)
}

func (p *chainedObjectParser[TReference, TParsedObject]) GetDecodingParametersSizeBytes() int {
	return p.parserA.GetDecodingParametersSizeBytes()
}
