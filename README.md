# User Interaction Report Server

This Go application provides a simple HTTP server to record and retrieve user interactions (reports, likes, dislikes) stored in an in-memory FrostDB database.

## Running the Server

1. **Prerequisites:** Ensure you have Go installed (version 1.18 or later recommended).
2. **Install Dependencies:** Open a terminal in the project root directory (`F:\git\user-report-db`) and run:

```bash
    go mod tidy
```

3. **Run the Server:**

```bash
    go run main.go
```

The server will start listening on `http://localhost:8069`.

**Note:** Since FrostDB is used in-memory by default in this example, all data will be lost when the server is stopped.

## API Endpoints

The server exposes the following RESTful API endpoints:

### 1. Record a User Report

* **Method:** `POST`
* **Path:** `/app/report`
* **Request Body (JSON):**

```json
    {
      "userId": "string",         // ID of the user submitting the report
      "reportedUserId": "string", // ID of the user being reported
      "message": "string"         // The content of the report message
    }
```

* **Description:** Stores a record indicating that `userId` reported `reportedUserId` with the provided `message`.
* **Success Response (201 Created):** Returns the created interaction record (including the generated timestamp).

```json
    {
      "userId": "user123",
      "reportedUserId": "user456",
      "type": "report",
      "message": "Spamming chat",
      "timestamp": 1678886400123456789 // Example UnixNano timestamp
    }
```

* **Error Responses:**
  * `400 Bad Request`: If the request body is invalid or missing required fields.
  * `405 Method Not Allowed`: If a method other than POST is used.
  * `500 Internal Server Error`: If there's an issue saving the data.

### 2. Record a User Like

* **Method:** `POST`
* **Path:** `/app/like`
* **Request Body (JSON):**

```json
    {
      "userId": "string",         // ID of the user performing the like
      "reportedUserId": "string" // ID of the user being liked
    }
```

* **Description:** Stores a record indicating that `userId` liked `reportedUserId`.
* **Success Response (201 Created):** Returns the created interaction record.

```json

    {
      "userId": "user789",
      "reportedUserId": "user101",
      "type": "like",
      "message": "", // Message is empty for likes/dislikes
      "timestamp": 1678886500123456789
    }
```

* **Error Responses:**
  * `400 Bad Request`: If the request body is invalid or missing required fields.
  * `405 Method Not Allowed`: If a method other than POST is used.
  * `500 Internal Server Error`: If there's an issue saving the data.

### 3. Record a User Dislike

* **Method:** `POST`
* **Path:** `/app/dislike`
* **Request Body (JSON):**

    ```json
    {
      "userId": "string",         // ID of the user performing the dislike
      "reportedUserId": "string" // ID of the user being disliked
    }
    ```

* **Description:** Stores a record indicating that `userId` disliked `reportedUserId`.
* **Success Response (201 Created):** Returns the created interaction record.

    ```json
    {
      "userId": "user222",
      "reportedUserId": "user333",
      "type": "dislike",
      "message": "",
      "timestamp": 1678886600123456789
    }
    ```

* **Error Responses:**
  * `400 Bad Request`: If the request body is invalid or missing required fields.
  * `405 Method Not Allowed`: If a method other than POST is used.
  * `500 Internal Server Error`: If there's an issue saving the data.

### 4. Get User Interactions

* **Method:** `GET`
* **Path:** `/app/user/{userId}/interactions/{direction}`
* **Path Parameters:**
  * `{userId}` (string): The ID of the user whose interactions you want to retrieve.
  * `{direction}` (string): Specifies the perspective. Must be either `sent` or `received`.
* **Request Body:** None
* **Description:**
  * If `direction` is `sent`, retrieves all interactions (reports, likes, dislikes) initiated *by* the specified `{userId}`.
  * If `direction` is `received`, retrieves all interactions (reports, likes, dislikes) targeted *at* the specified `{userId}`.
* **Success Response (200 OK):** Returns an array of interaction records matching the criteria. The array might be empty if no interactions are found.

    ```json
    [
      {
        "userId": "user123",
        "reportedUserId": "user456",
        "type": "report",
        "message": "Spamming chat",
        "timestamp": 1678886400123456789
      },
      {
        "userId": "user123",
        "reportedUserId": "user789",
        "type": "like",
        "message": "",
        "timestamp": 1678886700123456789
      }
      // ... other interactions
    ]
    ```

* **Error Responses:**
  * `404 Not Found`: If the URL path structure is incorrect (e.g., wrong number of segments, invalid direction).
  * `405 Method Not Allowed`: If a method other than GET is used.
  * `500 Internal Server Error`: If there's an issue retrieving the data.
