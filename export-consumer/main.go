package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/streadway/amqp"
)

type ExportMessage struct {
	ID        string `json:"id"`
	FilePath  string `json:"file_path"` //the reference to the generated csv file
	Timestamp int64 `json:"timestamp"`
}

type ExportProcessor struct {
}

func main() {

	processor := &ExportProcessor{}
	go processor.startConsumer()

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong pong",
		})
	})
	r.Run();
}

func (p *ExportProcessor) startConsumer() {

	// Connect to RabbitMQ server
	mqHost := os.Getenv("RABBITMQ_HOST")
	if mqHost == "" {
		mqHost = "amqp://guest:guest@event-dispatcher:5672/"
	}
	conn, err := amqp.Dial(mqHost)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
		//send error message to manager
		return
	}
	defer conn.Close()

	// Create a channel
	ch, err := conn.Channel()
	if err != nil {
		//send error message to manager
		log.Fatalf("Failed to open a channel: %v", err)
		return
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

	// Declare a topic exchange for exporting csv
	err = ch.ExchangeDeclare(
		"csv_export", // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)

	if err != nil {
		log.Fatalf("Failed to declare a topic exchange: %v", err)
	}

	// Declare a queue
	q, err := ch.QueueDeclare(
		"export-consumer", // name
		false,             // durable
		false,             // delete when unused
		true,              // exclusive
		false,             // no-wait
		nil,               // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare a queue: %v", err)
	}

	// Bind the queue to the exchange with a routing key
	err = ch.QueueBind(
		q.Name,         // queue name
		"csv.#.#",      // routing key
		"csv_exchange", // exchange
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		//handle error, send error message to manager
		log.Fatalf("Failed to bind the queue to the exchange: %v", err)
		return
	}

	// Consume messages from the queue
	msgs, err := ch.Consume(
		q.Name,            // queue name
		"export-consumer", // consumer name
		true,              // auto-ack
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // arguments
	)
	if err != nil {
		log.Fatalf("Failed to consume messages: %v", err)
		// handle error
		return
	}

	// Process incoming messages
	for msg := range msgs {
		log.Printf("Export-Consumer: received a message- %s", msg.Body)
		fmt.Println(msg.RoutingKey)

		parts := strings.Split(msg.RoutingKey, ".")
		if len(parts) != 3 {
			log.Printf("Invalid routing key format: %s", msg.RoutingKey)
			continue
		}

		
		var exportReq ExportMessage
		if err := json.Unmarshal(msg.Body, &exportReq); err != nil {
			log.Printf("Failed to parse export request: %v", err)
			msg.Ack(false)
			continue
		}


		exportFile := ExportMessage{
			ID:        parts[1],
			FilePath:  exportReq.FilePath,
			Timestamp: time.Now().Unix(),
		}

		// Convert the message to JSON
		exportBody, err := json.Marshal(exportFile)
		if err != nil {
			fmt.Errorf("failed to marshal export message: %v", err)
			continue
		}

		err = ch.Publish(
			"csv_export",                    // exchange
			fmt.Sprintf("req.%s", parts[1]), // routing key
			false,                           // mandatory
			false,                           // immediate
			amqp.Publishing{
				ContentType: "application/json",
				Body:        exportBody,
			},
		)

		if err != nil {
			log.Fatalf("Failed to publish a message: %v", err)
		}

		log.Println("Message sent successfully!--------------- from consumer adfsdf")
		log.Println("Deliever to others   -------------------- from cnsumer")
		log.Println("Save to database   -------------------- from cnsumer")
	}
}
