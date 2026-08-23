package milvus

import (
	"strconv"
	"strings"

	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	IndexTypeIVFFlat = "ivf_flat"
	IndexTypeHNSW    = "hnsw"

	MetricIP     = "IP"
	MetricL2     = "L2"
	MetricCosine = "COSINE"

	DefaultNList              = 128
	DefaultNProbe             = 16
	DefaultHNSWM              = 16
	DefaultHNSWEfConstruction = 200
	DefaultHNSWEf             = 64
)

type IndexConfig struct {
	IndexType          string
	Metric             string
	NList              int
	NProbe             int
	HNSWM              int
	HNSWEfConstruction int
	HNSWEf             int
	ScoreThreshold     float32
}

func DefaultIndexConfig() IndexConfig {
	return IndexConfig{
		IndexType:          IndexTypeIVFFlat,
		Metric:             MetricIP,
		NList:              DefaultNList,
		NProbe:             DefaultNProbe,
		HNSWM:              DefaultHNSWM,
		HNSWEfConstruction: DefaultHNSWEfConstruction,
		HNSWEf:             DefaultHNSWEf,
		ScoreThreshold:     0,
	}
}

func (c IndexConfig) MetricType() entity.MetricType {
	switch strings.ToUpper(strings.TrimSpace(c.Metric)) {
	case MetricL2:
		return entity.L2
	case MetricCosine:
		return entity.COSINE
	default:
		return entity.IP
	}
}

func (c IndexConfig) IndexSignature() string {
	return strings.Join([]string{
		strings.ToLower(c.IndexType),
		strings.ToUpper(c.Metric),
		strconv.Itoa(c.NList),
		strconv.Itoa(c.HNSWM),
		strconv.Itoa(c.HNSWEfConstruction),
	}, "|")
}

func (c IndexConfig) Normalized() IndexConfig {
	out := c
	if out.IndexType == "" {
		out.IndexType = IndexTypeIVFFlat
	}
	if out.Metric == "" {
		out.Metric = MetricIP
	}
	if out.NList <= 0 {
		out.NList = DefaultNList
	}
	if out.NProbe <= 0 {
		out.NProbe = DefaultNProbe
	}
	if out.HNSWM <= 0 {
		out.HNSWM = DefaultHNSWM
	}
	if out.HNSWEfConstruction <= 0 {
		out.HNSWEfConstruction = DefaultHNSWEfConstruction
	}
	if out.HNSWEf <= 0 {
		out.HNSWEf = DefaultHNSWEf
	}
	return out
}
