package exporter

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
)

type IWriter interface {
    Write(data interface{})
}

type IFileWriter interface {
    IWriter
}
type ICSVFileWriter struct {

}

type IExporter interface {
	Export(data interface{}) ([]byte, error)
}

type ICSVExporter interface {
	IExporter
}

type JSONToCSVExporter struct {
}

func (c *JSONToCSVExporter) Export(data interface{}) ([]byte, error) {
    return nil, nil
}


type CSVExporterFactory struct{}

func (f *CSVExporterFactory) GetExporterFromFile(filePath string) (ICSVExporter, error) {
	// Determine file type and return approciate csvexporter
	return nil, nil
}
