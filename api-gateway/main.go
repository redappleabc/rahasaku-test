package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/streadway/amqp"
)

type ExportResponse struct {
	ID        string `json:"id"`
	FilePath  string `json:"file_path"` //the reference to the generated csv file
	Timestamp int64  `json:"timestamp"`
}

type ExportRequest struct {
	ID        string          `json:"id"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// Delevery Processor
type ExportProcessor struct {
	requests map[string]ExportResponse
	mu       sync.Mutex
}

func (p *ExportProcessor) Publish(req ExportResponse) {
	// Save the delevery request to the dictionary
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests[req.ID] = req
}

func (p *ExportProcessor) Consume(id string, callback func(bool, string)) {
	// Wait for the requested ID to be received or for 1 minute to pass
	start := time.Now()
	for {
		p.mu.Lock()
		if req, ok := p.requests[id]; ok {
			delete(p.requests, id)
			p.mu.Unlock()
			callback(false, req.FilePath)
			return
		}
		p.mu.Unlock()

		if time.Since(start) > 15*time.Second {
			log.Printf("Timeout waiting for request with ID %s", id)
			callback(true, "")
			return
		}

		time.Sleep(time.Second)
	}
}

func (p *ExportProcessor) Run() {
	// Set up RabbitMQ connection
	mqHost := os.Getenv("RABBITMQ_HOST")
	if mqHost == "" {
		mqHost = "amqp://guest:guest@event-dispatcher:5672/"
	}
	conn, err := amqp.Dial(mqHost)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	// Set up channel for receiving responses
	respChannel, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}

	// Declare the response queue
	respQueue, err := respChannel.QueueDeclare(
		"csv_export", // name
		false,        // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare a queue: %v", err)
	}

	// Bind the queue to the exchange with a routing key
	err = respChannel.QueueBind(
		respQueue.Name, // queue name
		"req.#",        // routing key
		"csv_export",   // exchange
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		log.Fatalf("Failed to bind the queue to the exchange: %v", err)
	}

	// Set up consumer for response queue
	respConsumer, err := respChannel.Consume(
		respQueue.Name,            // queue
		fmt.Sprint("api-gateway"), // consumer
		false,                     // auto-ack
		false,                     // exclusive
		false,                     // no-local
		false,                     // no-wait
		nil,                       // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	defer respChannel.Close()

	// Process delivery requests as they are received
	for resp := range respConsumer {
		log.Printf("API-GATEWAY: message received")
		var exportReq ExportResponse
		if err := json.Unmarshal(resp.Body, &exportReq); err != nil {
			log.Printf("Failed to parse export request: %v", err)
			resp.Ack(false)
			continue
		}
		// Update the delivery processor with the new request
		p.Publish(exportReq)
		resp.Ack(false)
	}
}

func main() {
	
	exportServiceHost := os.Getenv("EXPORT_SERVICE_HOST")
	if exportServiceHost == "" {
		exportServiceHost = "http://export-service:8080"
	}
	processor := &ExportProcessor{
		requests: make(map[string]ExportResponse),
	}
	// Start the processor
	go processor.Run()

	// Set up API Gateway
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.POST("/export", func(c *gin.Context) {

		// Authorize if user has more than 1 credits.

		// Generate a unique ID for the request
		reqID := uuid.New().String()
		timestamp := time.Now().Unix()

		// Read the original request body
		reqBodyBytes, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Create the export request
		exportReq := ExportRequest{
			ID:        reqID,
			Timestamp: timestamp,
			Data:      json.RawMessage(reqBodyBytes),
		}

		exportReqBytes, err := json.Marshal(exportReq)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Send the export request to the export service
		resp, err := http.Post(fmt.Sprintf("%s/export", exportServiceHost), "application/json", bytes.NewBuffer(exportReqBytes))

		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Export request failed"})
			return
		}

		fmt.Println(fmt.Sprintf("csv.%s", reqID))

		callBackFunc := func(timeouted bool, filePath string) {
			// Return the export response
			if timeouted {
				c.AbortWithStatusJSON(http.StatusRequestTimeout, gin.H{
					"message": "Time out error",
				})
			}
			//save to database
			c.Header("Content-Disposition", "attachment; filename="+filePath)
			c.Header("Content-Type", "text/csv")
			c.File(filePath)
			// c.JSON(http.StatusOK, gin.H{
			// 	"message": filePath,
			// })

			log.Printf("Received request with ID %s", filePath)
			return
		}
		// Consume the reqID
		processor.Consume(reqID, callBackFunc)

	})
	r.Run() // listen and serve on 0.0.0.0:8080
}
