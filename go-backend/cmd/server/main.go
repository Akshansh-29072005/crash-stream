package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hibiken/asynq"

	"traffic-accident-detection-mvp/go-backend/tasks"
)

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for simplicity; adjust in production
	},
}

// ClientManager manages WebSocket clients for broadcasting
type ClientManager struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
}

var manager = ClientManager{
	clients:    make(map[*websocket.Conn]bool),
	broadcast:  make(chan []byte),
	register:   make(chan *websocket.Conn),
	unregister: make(chan *websocket.Conn),
}

// AccidentResult represents the structure of accident detection results
type AccidentResult struct {
	AccidentDetected bool    `json:"accident_detected"`
	Confidence       float64 `json:"confidence"`
	Bbox             []float64 `json:"bbox,omitempty"`
	Timestamp        string  `json:"timestamp"`
}

func main() {
	r := gin.Default()

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// Initialize Asynq Client
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	defer client.Close()

	// API routes
	r.GET("/api/accidents", handleAccidents)
	r.POST("/api/accidents", handleAccidentDetection)
	r.POST("/api/detection-result", handleDetectionResult)
	r.POST("/api/stream", func(c *gin.Context) {
		handleStream(c, client)
	})
	r.GET("/ws/alerts", handleWebSocket)

	// Serve the index.html at the root
	r.GET("/", func(c *gin.Context) {
		c.File("./web/index.html")
	})

	// Start the client manager goroutine
	go manager.start()

	// Start Asynq Worker
	go startAsynqWorker(redisAddr)

	// Start the server
	log.Println("Starting server on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Could not start server: %v\n", err)
	}
}

// handleAccidents returns accident data (placeholder)
func handleAccidents(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Accident data",
	})
}

// handleAccidentDetection triggers accident detection (placeholder)
// In a real implementation, this would receive a frame, process it, and broadcast results
func handleAccidentDetection(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Accident detection triggered",
	})
}

// handleDetectionResult receives accident detection results from Python backend and broadcasts them
func handleDetectionResult(c *gin.Context) {
	var result AccidentResult
	if err := c.ShouldBindJSON(&result); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert result to JSON for broadcasting
	jsonData, err := json.Marshal(result)
	if err != nil {
		log.Printf("Error marshaling accident result: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process result"})
		return
	}

	// Broadcast to all WebSocket clients
	manager.broadcast <- jsonData

	c.JSON(http.StatusOK, gin.H{"message": "Result broadcasted"})
}

// handleWebSocket upgrades the connection and manages the WebSocket client
func handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v\n", err)
		return
	}
	defer conn.Close()

	// Register the new client
	manager.register <- conn

	// Ensure that when the function returns, the client is unregistered
	defer func() {
		manager.unregister <- conn
	}()

	// Listen for messages from the client (if any)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v\n", err)
			}
			break
		}
	}
}

// start manages the client registry and broadcasts messages
func (manager *ClientManager) start() {
	for {
		select {
		case conn := <-manager.register:
			manager.clients[conn] = true
			log.Println("New WebSocket client connected")
		case conn := <-manager.unregister:
			if _, ok := manager.clients[conn]; ok {
				delete(manager.clients, conn)
				conn.Close()
				log.Println("WebSocket client disconnected")
			}
		case message := <-manager.broadcast:
			for conn := range manager.clients {
				if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
					log.Printf("WebSocket write error: %v\n", err)
					conn.Close()
					delete(manager.clients, conn)
				}
			}
		}
	}
}

// handleStream receives a frame from the phone and enqueues a detection task
func handleStream(c *gin.Context, client *asynq.Client) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer f.Close()

	imgData, err := ioutil.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	task, err := tasks.NewDetectionTask(imgData, time.Now().Format(time.RFC3339))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task"})
		return
	}

	_, err = client.Enqueue(task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue task"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "queued"})
}

// startAsynqWorker starts the asynq worker to process detection tasks
func startAsynqWorker(redisAddr string) {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 10,
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeAccidentDetection, HandleDetectionTask)

	log.Printf("Starting Asynq worker on %s\n", redisAddr)
	if err := srv.Run(mux); err != nil {
		log.Fatalf("could not run asynq server: %v", err)
	}
}

// HandleDetectionTask processes the detection task by calling the Python backend
func HandleDetectionTask(ctx context.Context, t *asynq.Task) error {
	var p tasks.DetectionTaskPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	pythonURL := os.Getenv("PYTHON_BACKEND_URL")
	if pythonURL == "" {
		pythonURL = "http://localhost:5000"
	}

	// Prepare multipart request to Python
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "frame.jpg")
	if err != nil {
		return err
	}
	part.Write(p.ImageData)
	writer.Close()

	req, err := http.NewRequest("POST", pythonURL+"/detect", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("python backend returned status: %d", resp.StatusCode)
	}

	// The Python backend is expected to send the result back to /api/detection-result
	// which will handle the WebSocket broadcast.
	return nil
}
