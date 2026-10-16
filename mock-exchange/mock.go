package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Order represents an incoming client order
type Order struct {
	OrderID   string  `json:"order_id"`
	Side      string  `json:"side"`  // "BUY" or "SELL"
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	Timestamp int64   `json:"timestamp"`
}

// ExecutionAck is sent back to the client
type ExecutionAck struct {
	OrderID     string  `json:"order_id"`
	Status      string  `json:"status"` // "ACK" or "FILLED"
	MatchedWith string  `json:"matched_with,omitempty"`
	ExecPrice   float64 `json:"exec_price,omitempty"`
	ExecQty     int     `json:"exec_qty,omitempty"`
	ServerTime  int64   `json:"server_time"`
}

// OrderBook maintains bids and asks in memory
type OrderBook struct {
	bids []Order // sorted descending by price
	asks []Order // sorted ascending by price
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		bids: make([]Order, 0),
		asks: make([]Order, 0),
	}
}

// ProcessOrder matches an order or places it in the book
func (ob *OrderBook) ProcessOrder(incoming Order) []ExecutionAck {
	var events []ExecutionAck
	now := time.Now().UnixNano()

	if incoming.Side == "BUY" {
		// Try matching against asks (lowest price first)
		for len(ob.asks) > 0 && incoming.Quantity > 0 {
			bestAsk := &ob.asks[0]
			if incoming.Price < bestAsk.Price {
				break
			}

			tradeQty := min(incoming.Quantity, bestAsk.Quantity)
			incoming.Quantity -= tradeQty
			bestAsk.Quantity -= tradeQty

			events = append(events, ExecutionAck{
				OrderID:     incoming.OrderID,
				Status:      "FILLED",
				MatchedWith: bestAsk.OrderID,
				ExecPrice:   bestAsk.Price,
				ExecQty:     tradeQty,
				ServerTime:  now,
			})

			if bestAsk.Quantity == 0 {
				ob.asks = ob.asks[1:]
			}
		}

		// Remaining resting order
		if incoming.Quantity > 0 {
			ob.bids = append(ob.bids, incoming)
			sort.Slice(ob.bids, func(i, j int) bool {
				return ob.bids[i].Price > ob.bids[j].Price // Highest price priority
			})
			events = append(events, ExecutionAck{
				OrderID:    incoming.OrderID,
				Status:     "ACK",
				ServerTime: now,
			})
		}
	} else if incoming.Side == "SELL" {
		// Try matching against bids (highest price first)
		for len(ob.bids) > 0 && incoming.Quantity > 0 {
			bestBid := &ob.bids[0]
			if incoming.Price > bestBid.Price {
				break
			}

			tradeQty := min(incoming.Quantity, bestBid.Quantity)
			incoming.Quantity -= tradeQty
			bestBid.Quantity -= tradeQty

			events = append(events, ExecutionAck{
				OrderID:     incoming.OrderID,
				Status:      "FILLED",
				MatchedWith: bestBid.OrderID,
				ExecPrice:   bestBid.Price,
				ExecQty:     tradeQty,
				ServerTime:  now,
			})

			if bestBid.Quantity == 0 {
				ob.bids = ob.bids[1:]
			}
		}

		// Remaining resting order
		if incoming.Quantity > 0 {
			ob.asks = append(ob.asks, incoming)
			sort.Slice(ob.asks, func(i, j int) bool {
				return ob.asks[i].Price < ob.asks[j].Price // Lowest price priority
			})
			events = append(events, ExecutionAck{
				OrderID:    incoming.OrderID,
				Status:     "ACK",
				ServerTime: now,
			})
		}
	}

	return events
}

type ExchangeServer struct {
	orderChan chan OrderJob
	book      *OrderBook
}

type OrderJob struct {
	Order Order
	AckCh chan []ExecutionAck
}

func NewExchangeServer(bufferSize int) *ExchangeServer {
	es := &ExchangeServer{
		orderChan: make(chan OrderJob, bufferSize),
		book:      NewOrderBook(),
	}
	go es.runMatchingEngine()
	return es
}

// Single-threaded worker loop handles order matching sequentially without lock contention
func (es *ExchangeServer) runMatchingEngine() {
	for job := range es.orderChan {
		acks := es.book.ProcessOrder(job.Order)
		job.AckCh <- acks
	}
}

var exchangeServer = NewExchangeServer(10000)

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var order Order
		if err := json.Unmarshal(message, &order); err != nil {
			continue
		}

		ackCh := make(chan []ExecutionAck, 1)
		exchangeServer.orderChan <- OrderJob{
			Order: order,
			AckCh: ackCh,
		}
		acks := <-ackCh

		for _, ack := range acks {
			responseBytes, _ := json.Marshal(ack)
			if err := conn.WriteMessage(websocket.TextMessage, responseBytes); err != nil {
				return
			}
		}
	}
}

func main() {
	http.HandleFunc("/ws", wsHandler)
	log.Println("Mock Trading Engine listening on ws://localhost:8080/ws")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}