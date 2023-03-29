package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streadway/amqp"
)

type IExporter interface {
	Export(data interface{}) ([]byte, error)
}

type ICSVExporter interface {
	IExporter
}

type JSONToCSVExporter struct {
}

func (c *JSONToCSVExporter) Export(data interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var rows []map[string]interface{}
	err = json.Unmarshal(jsonData, &rows)
	if err != nil {
		return nil, err
	}

	var csvData bytes.Buffer
	csvWriter := csv.NewWriter(&csvData)

	for i, row := range rows {
		if i == 0 {
			headers := make([]string, 0, len(row))
			for header := range row {
				headers = append(headers, header)
			}
			err = csvWriter.Write(headers)
			if err != nil {
				return nil, err
			}
		}

		values := make([]string, 0, len(row))
		for _, value := range row {
			switch v := value.(type) {
			case string:
				values = append(values, v)
			case int:
				values = append(values, strconv.Itoa(v))
			case float64:
				values = append(values, strconv.FormatFloat(v, 'f', -1, 64))
			default:
				values = append(values, fmt.Sprintf("%v", v))
			}
		}

		err = csvWriter.Write(values)
		if err != nil {
			fmt.Println("")
			return nil, err
		}
	}

	csvWriter.Flush()

	if csvWriter.Error() != nil {
		return nil, csvWriter.Error()
	}

	return csvData.Bytes(), nil
}

type ExportRequested struct {
	ID        string          `json:"id"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type ExportMessage struct {
	ID        string `json:"id"`
	FilePath  string `json:"file_path"` //the reference to the generated csv file
	Timestamp int64  `json:"timestamp"`
}

func main() {

	// Connect to RabbitMQ server
	mqHost := os.Getenv("RABBITMQ_HOST")
	if mqHost == "" {
		mqHost = "amqp://guest:guest@event-dispatcher:5672/"
	}
	conn, err := amqp.Dial(mqHost)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	// Create a channel
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	// Declare a topic exchange
	err = ch.ExchangeDeclare(
		"csv_exchange", // name
		"topic",        // type
		true,           // durable
		false,          // auto-deleted
		false,          // internal
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare a topic exchange: %v", err)
	}

	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong pong",
		})
	})

	r.POST("/export", func(c *gin.Context) {
		// Parse the export request from the request body
		var exportReq ExportRequested
		if err := c.BindJSON(&exportReq); err != nil {
			fmt.Println("Request body:", err.Error())
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse export request"})
			return
		}
		c.JSON(200, gin.H{
			"message": "successfully received request",
		})

		dirname := "/csvfiles"
		if _, err := os.Stat(dirname); errors.Is(err, os.ErrNotExist) {
			err := os.Mkdir(dirname, os.ModePerm)
			if err != nil {
				log.Println(err)
				//handle error, maybe send error message to export consumer
			}
		}

		exporter := &JSONToCSVExporter{}
		csvData, err := exporter.Export(exportReq.Data)

		filename := fmt.Sprintf("%s/%s--%d-%d.csv", dirname, exportReq.ID, exportReq.Timestamp, time.Now().Unix())
		err = ioutil.WriteFile(filename, csvData, 0644)
		if err != nil {
			fmt.Println("file save error ----------------------- export-service")
			//do something for exception. maybe send to err message to export-consumer
			panic(err)
		}

		csv_data := ExportMessage{
			Timestamp: exportReq.Timestamp,
			ID:        exportReq.ID,
			FilePath:  filename,
		}

		exportBody, err := json.Marshal(csv_data)
		if err != nil {
			log.Fatalf("Failed to marshal export message: %v", err)
		}

		err = ch.Publish(
			"csv_exchange", // exchange
			fmt.Sprintf("csv.%s.%d", exportReq.ID, exportReq.Timestamp), // routing key
			false, // mandatory
			false, // immediate
			amqp.Publishing{
				ContentType: "application/json",
				Body:        exportBody,
			},
		)
		if err != nil {
			log.Fatalf("Failed to publish a message: %v", err)
			//handle error. send message to manager
		}

		log.Println("Message sent successfully!")

	})

	r.Run() // listen and serve on 0.0.0.0:8080
}
