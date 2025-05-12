package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/polarsignals/frostdb"
	"github.com/polarsignals/frostdb/query"
	"github.com/polarsignals/frostdb/query/logicalplan"
)

const (
	dbName    = "user_interactions_db"
	tableName = "interactions_table"
	port      = ":8069"

	InteractionTypeReport  = "report"
	InteractionTypeLike    = "like"
	InteractionTypeDislike = "dislike"
)

// InteractionRecord holds the data for different user interactions.
// Using tags for FrostDB schema definition.
type InteractionRecord struct {
	UserID         string `json:"userId" frostdb:",rle_dict,asc(0)"`           // User initiating the action
	ReportedUserID string `json:"reportedUserId" frostdb:",rle_dict,asc(1)"` // User receiving the action
	Type           string `json:"type" frostdb:",rle_dict,asc(2)"`           // Type of interaction: report, like, dislike
	Message        string `json:"message,omitempty"`                         // Optional message, mainly for reports
	Timestamp      int64  `json:"timestamp" frostdb:",asc(3)"`               // Time of the interaction (UnixNano)
}

var db *frostdb.DB
var table *frostdb.GenericTable[InteractionRecord]
var queryEngine *query.LocalEngine

func main() {
	// Initialize FrostDB
	// Configure persistence options
	opts := []frostdb.Option{
		frostdb.WithWAL(),                 // Enable Write-Ahead Log for durability
		frostdb.WithStoragePath("frostdb_data"), // Specify the directory for data files
	}
	columnstore, err := frostdb.New(opts...)
	if err != nil {
		log.Fatalf("Failed to create column store: %v", err)
	}
	defer columnstore.Close()

	database, err := columnstore.DB(context.Background(), dbName)
	if err != nil {
		log.Fatalf("Failed to get database '%s': %v", dbName, err)
	}
	db = database

	// Create or get the table
	interactionsTable, err := frostdb.NewGenericTable[InteractionRecord](
		db, tableName, memory.DefaultAllocator,
	)
	if err != nil {
		log.Fatalf("Failed to create/get table '%s': %v", tableName, err)
	}
	table = interactionsTable
	// Note: Table should be released when the application exits.
	// In a real application, use signal handling for graceful shutdown.
	// defer table.Release()

	// Initialize the query engine
	queryEngine = query.NewEngine(memory.DefaultAllocator, db.TableProvider())

	// Setup HTTP routes
	http.HandleFunc("/app/report", handleReport)
	http.HandleFunc("/app/like", handleLike)
	http.HandleFunc("/app/dislike", handleDislike)
	http.HandleFunc("/app/user/", handleGetUserInteractions) // Catch-all for user interactions

	// Start the server
	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// Placeholder for handler functions - to be implemented next
func handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		UserID         string `json:"userId"`
		ReportedUserID string `json:"reportedUserId"`
		Message        string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Printf("Error decoding report request body: %v", err)
		sendJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if reqBody.UserID == "" || reqBody.ReportedUserID == "" || reqBody.Message == "" {
		sendJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing required fields: userId, reportedUserId, message"})
		return
	}

	interaction := InteractionRecord{
		UserID:         reqBody.UserID,
		ReportedUserID: reqBody.ReportedUserID,
		Type:           InteractionTypeReport,
		Message:        reqBody.Message,
		Timestamp:      time.Now().UnixNano(), // Set timestamp explicitly
	}

	if err := writeInteraction(r.Context(), interaction); err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save report"})
		return
	}

	sendJSONResponse(w, http.StatusCreated, interaction)
}

func handleLike(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		UserID         string `json:"userId"`
		ReportedUserID string `json:"reportedUserId"` // Liked User ID
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Printf("Error decoding like request body: %v", err)
		sendJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if reqBody.UserID == "" || reqBody.ReportedUserID == "" {
		sendJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing required fields: userId, reportedUserId"})
		return
	}

	interaction := InteractionRecord{
		UserID:         reqBody.UserID,
		ReportedUserID: reqBody.ReportedUserID,
		Type:           InteractionTypeLike,
		Timestamp:      time.Now().UnixNano(),
	}

	if err := writeInteraction(r.Context(), interaction); err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save like"})
		return
	}

	sendJSONResponse(w, http.StatusCreated, interaction)
}

func handleDislike(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody struct {
		UserID         string `json:"userId"`
		ReportedUserID string `json:"reportedUserId"` // Disliked User ID
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Printf("Error decoding dislike request body: %v", err)
		sendJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if reqBody.UserID == "" || reqBody.ReportedUserID == "" {
		sendJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing required fields: userId, reportedUserId"})
		return
	}

	interaction := InteractionRecord{
		UserID:         reqBody.UserID,
		ReportedUserID: reqBody.ReportedUserID,
		Type:           InteractionTypeDislike,
		Timestamp:      time.Now().UnixNano(),
	}

	if err := writeInteraction(r.Context(), interaction); err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save dislike"})
		return
	}

	sendJSONResponse(w, http.StatusCreated, interaction)
}

func handleGetUserInteractions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Expected path format: /app/user/{userId}/interactions/(sent|received)
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/app/user/"), "/")
	if len(pathParts) != 3 || pathParts[1] != "interactions" || (pathParts[2] != "sent" && pathParts[2] != "received") {
		log.Printf("Invalid user interactions path: %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	userID := pathParts[0]
	direction := pathParts[2] // "sent" or "received"

	var filter logicalplan.Expr
	if direction == "sent" {
		// Filter by the user who initiated the interaction
		filter = logicalplan.Col("userId").Eq(logicalplan.Literal(userID))
	} else { // direction == "received"
		// Filter by the user who received the interaction
		filter = logicalplan.Col("reportedUserId").Eq(logicalplan.Literal(userID))
	}

	results, err := queryInteractions(r.Context(), filter)
	if err != nil {
		sendJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to retrieve interactions"})
		return
	}

	sendJSONResponse(w, http.StatusOK, results)
}

// --- Helper Functions (to be implemented) ---

// writeInteraction saves an interaction record to FrostDB
func writeInteraction(ctx context.Context, record InteractionRecord) error {
	// Timestamp is expected to be set by the caller (handlers)
	// if record.Timestamp == 0 {
	// 	record.Timestamp = time.Now().UnixNano()
	// }

	_, err := table.Write(ctx, record)
	if err != nil {
		log.Printf("Error writing interaction to FrostDB: %v", err)
		return fmt.Errorf("failed to write interaction: %w", err)
	}
	log.Printf("Successfully wrote interaction: %+v", record)
	return nil
}

// queryInteractions retrieves interactions based on filters
func queryInteractions(ctx context.Context, filter logicalplan.Expr) ([]InteractionRecord, error) {
	var results []InteractionRecord = make([]InteractionRecord, 0) // Initialize as an empty slice
	var queryBuilder query.Builder

	scan := queryEngine.ScanTable(tableName)
	if filter != nil {
		queryBuilder = scan.Filter(filter)
	} else {
		queryBuilder = scan
	}

	err := queryBuilder.Execute(ctx, func(ctx context.Context, r arrow.Record) error {
		defer r.Release()
		for i := 0; i < int(r.NumRows()); i++ {
			record, err := arrowRecordToInteraction(r, i)
			if err != nil {
				log.Printf("Error converting arrow row %d to InteractionRecord: %v", i, err)
				// Decide whether to skip the row or return an error
				// Skipping for now
				continue
			}
			results = append(results, record)
		}
		return nil
	})

	if err != nil {
		log.Printf("Error executing query: %v", err)
		return nil, fmt.Errorf("failed to query interactions: %w", err)
	}

	return results, nil
}

// arrowRecordToInteraction converts an arrow record row to InteractionRecord
// This needs careful implementation based on the exact FrostDB schema and arrow types.
func arrowRecordToInteraction(rec arrow.Record, row int) (InteractionRecord, error) {
	interaction := InteractionRecord{}
	schema := rec.Schema()

	for i, field := range schema.Fields() {
		col := rec.Column(i)
		if col.IsNull(row) {
			continue // Skip null fields
		}

		switch field.Name {
		case "userId":
			if dictArr, ok := col.(*array.Dictionary); ok {
				if strArr, ok := dictArr.Dictionary().(*array.String); ok {
					interaction.UserID = strArr.Value(dictArr.GetValueIndex(row))
				} else {
					return InteractionRecord{}, fmt.Errorf("unexpected dictionary type for userId: %T", dictArr.Dictionary())
				}
			} else {
				return InteractionRecord{}, fmt.Errorf("unexpected array type for userId: %T", col)
			}
		case "reportedUserId":
			if dictArr, ok := col.(*array.Dictionary); ok {
				if strArr, ok := dictArr.Dictionary().(*array.String); ok {
					interaction.ReportedUserID = strArr.Value(dictArr.GetValueIndex(row))
				} else {
					return InteractionRecord{}, fmt.Errorf("unexpected dictionary type for reportedUserId: %T", dictArr.Dictionary())
				}
			} else {
				return InteractionRecord{}, fmt.Errorf("unexpected array type for reportedUserId: %T", col)
			}
		case "type":
			if dictArr, ok := col.(*array.Dictionary); ok {
				if strArr, ok := dictArr.Dictionary().(*array.String); ok {
					interaction.Type = strArr.Value(dictArr.GetValueIndex(row))
				} else {
					return InteractionRecord{}, fmt.Errorf("unexpected dictionary type for type: %T", dictArr.Dictionary())
				}
			} else {
				return InteractionRecord{}, fmt.Errorf("unexpected array type for type: %T", col)
			}
		case "message":
			if strArr, ok := col.(*array.String); ok {
				interaction.Message = strArr.Value(row)
			} else {
				return InteractionRecord{}, fmt.Errorf("unexpected array type for message: %T", col)
			}
		case "timestamp":
			// Expect Int64 for Unix nanoseconds
			if intArr, ok := col.(*array.Int64); ok {
				interaction.Timestamp = intArr.Value(row)
			} else {
				return InteractionRecord{}, fmt.Errorf("unexpected array type for timestamp (expected Int64): %T", col)
			}
		default:
			log.Printf("Warning: Unhandled column name '%s' during conversion", field.Name)
		}
	}

	return interaction, nil
}

// sendJSONResponse sends a JSON response
func sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
			// Attempt to send a plain error message if JSON encoding fails
			http.Error(w, `{"error":"Failed to encode response"}`, http.StatusInternalServerError)
		}
	}
} 