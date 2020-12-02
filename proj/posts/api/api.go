package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"database/sql"
	"strconv"
)


func RegisterRoutes(router *mux.Router) error {
	// Why don't we put options here? Check main.go :)

	router.HandleFunc("/api/posts/{startIndex}", getFeed).Methods(http.MethodGet, http.MethodOptions)
	router.HandleFunc("/api/posts/{uuid}/{startIndex}", getPosts).Methods(http.MethodGet, http.MethodOptions)
	router.HandleFunc("/api/posts/create", createPost).Methods(http.MethodPost, http.MethodOptions)
	router.HandleFunc("/api/posts/delete/{postID}", deletePost).Methods(http.MethodDelete, http.MethodOptions)

	return nil
}

func getUUID (w http.ResponseWriter, r *http.Request) (uuid string) {
	cookie, err := r.Cookie("access_token")
	if err != nil {
		http.Error(w, errors.New("error obtaining cookie: " + err.Error()).Error(), http.StatusBadRequest)
		log.Print(err.Error())
		return
	}
	//validate the cookie
	claims, err := ValidateToken(cookie.Value)
	if err != nil {
		http.Error(w, errors.New("error validating token: " + err.Error()).Error(), http.StatusUnauthorized)
		log.Print(err.Error())
		return
	}
	log.Println(claims)

	return claims["UserID"].(string)
}

func getPosts(w http.ResponseWriter, r *http.Request) {

	// Load the uuid and startIndex from the url parameter into their own variables
	// Look at mux.Vars() ... -> https://godoc.org/github.com/gorilla/mux#Vars
	// make sure to use "strconv" to convert the startIndex to an integer!
	// YOUR CODE HERE
	vars := mux.Vars(r)
	startIndex, convErr := strconv.Atoi(vars["startIndex"])
	urlUUID := vars["uuid"]
	if convErr != nil {
		http.Error(w, errors.New("error in converting string to int").Error(), http.StatusBadRequest)
		log.Print(convErr.Error())
		return
	}


	// Check if the user is authorized
	// First get the uuid from the access_token (see getUUID())
	// Compare that to the uuid we got from the url parameters, if they're not the same, return an error http.StatusUnauthorized
	// YOUR CODE HERE
	uuid := getUUID(w, r)
	if urlUUID != uuid {
		http.Error(w, errors.New("UUIDs do not match").Error(), http.StatusUnauthorized)
		return
	}

	var posts *sql.Rows
	var err error

	/*
		-Get all the posts that match our userID (or uuid)
		-Sort them chronologically (the database has a "postTime" field), hint: ORDER BY
		-Make sure to always get up to 25, and start with an offset of {startIndex} (look at the previous SQL homework for hints)\
		-As indicated by the "posts" variable, this query returns multiple rows
	*/
	posts, err = DB.Query("SELECT * FROM posts WHERE authorID = ? ORDER BY postTime LIMIT ?, 25", urlUUID, startIndex)

	// Check for errors from the query
	// YOUR CODE HERE
	if err != nil {
		http.Error(w, errors.New("error in getting all the posts that match the uuid").Error(), http.StatusInternalServerError)
		log.Print(err.Error())
		return
	}


	var (
		content string
		postID string
		userid string
		postTime time.Time
	)
	numPosts := 0
	// Create "postsArray", which is a slice (array) of Posts. Make sure it has size 25
	// Hint: https://tour.golang.org/moretypes/13
	postsArray := make([]Post, 25)

	for i := 0; i < 25 && posts.Next(); i++ {
		// Every time we call posts.Next() we get access to the next row returned from our query
		// Question: How many columns did we return
		// Reminder: Scan() scans the rows in order of their columns. See the variables defined up above for your convenience
		err = posts.Scan(&content, &postID, &userid, &postTime)

		// Check for errors in scanning
		// YOUR CODE HERE
		if err != nil {
			http.Error(w, errors.New("error in scanning query contents into variables").Error(), http.StatusInternalServerError)
			log.Print(err.Error())
			return
		}

		// Set the i-th index of postsArray to a new Post with values directly from the variables you just scanned into
		// Check post.go for the structure of a Post
		// Hint: https://gobyexample.com/structs
		//YOUR CODE HERE
		postsArray[i] = Post{PostBody: content, PostID: postID, AuthorID: userid, PostTime: postTime, PostAuthor: userid}

		numPosts++
	}

	posts.Close()
	err = posts.Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err.Error())
	}
  //encode fetched data as json and serve to client
  // Up until now, we've actually been counting the number of posts (numPosts)
  // We will always have *up to* 25 posts, but we can have less
  // However, we already allocated 25 spots in our postsArray
  // Return the subarray that contains all of our values (which may be a subsection of our array or the entire array)
  json.NewEncoder(w).Encode(postsArray[:numPosts])
  return;
}

func createPost(w http.ResponseWriter, r *http.Request) {
	// Obtain the userID from the JSON Web Token
	// See getUUID(...)
	// YOUR CODE HERE
	userID := getUUID(w, r)

	// Create a Post object and then Decode the JSON Body (which has the structure of a Post) into that object
	// YOUR CODE HERE
	post := Post{}
	decodeErr := json.NewDecoder(r.Body).Decode(&post)

	if decodeErr != nil {
		http.Error(w, errors.New("error in decoding Post from request body").Error(), http.StatusInternalServerError)
		log.Print(decodeErr.Error())
		return
	}

	// Use the uuid library to generate a post ID
	// Hint: https://godoc.org/github.com/google/uuid#New
	postID := uuid.New()

	//Load our location in PST
	pst, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	currPST := time.Now().In(pst)

	// Insert the post into the database
	// Look at /db-server/initdb.sql for a better understanding of what you need to insert
	result , e := DB.Exec("INSERT INTO posts (content, postID, authorID, postTime) VALUES (?, ?, ?, ?)", post.PostBody, postID, userID, currPST)

	// Check errors with executing the query
	// YOUR CODE HERE
	if e != nil {
		http.Error(w, e.Error(), http.StatusInternalServerError)
		log.Print(e.Error())
		return
	}

	// Make sure at least one row was affected, otherwise return an InternalServerError
	// You did something very similar in Checkpoint 2
	// YOUR CODE HERE
	n, error := result.RowsAffected()
	if error != nil {
		http.Error(w, error.Error(), http.StatusBadRequest)
		log.Print(error.Error())
		return
	}
	if n == 0 {
		http.Error(w, "500", http.StatusInternalServerError)
		return
	}

	// What kind of HTTP header should we return since we created something?
	// Check your signup from Checkpoint 2!
	// YOUR CODE HERE
	w.WriteHeader(http.StatusCreated)
  w.Write([]byte("201"))

	return
}

func deletePost(w http.ResponseWriter, r *http.Request) {

	// Get the postID to delete
	// Look at mux.Vars() ... -> https://godoc.org/github.com/gorilla/mux#Vars
	// YOUR CODE HERE
	postID := mux.Vars(r)["postID"]

	// Get the uuid from the access token, see getUUID(...)
	// YOUR CODE HERE
	uuid := getUUID(w, r)

	var exists bool
	//check if post exists
	err := DB.QueryRow("SELECT EXISTS(SELECT * FROM posts WHERE postID = ?)", postID).Scan(&exists)

	// Check for errors in executing the query
	// YOUR CODE HERE
	if err != nil {
		http.Error(w, errors.New("error in checking postID exists").Error(), http.StatusInternalServerError)
		log.Print(err.Error())
		return
	}

	// Check if the post actually exists, otherwise return an http.StatusNotFound
	// YOUR CODE HERE
	if !exists {
		http.Error(w, errors.New("this postID does not exist").Error(), http.StatusNotFound)
		return
	}

	// Get the authorID of the post with the specified postID
	var authorID string
	err = DB.QueryRow("SELECT authorID FROM posts WHERE postID = ?", postID).Scan(&authorID)

	// Check for errors in executing the query
	// YOUR CODE HERE
	if err != nil {
		http.Error(w, errors.New("error in getting authorID with postID").Error(), http.StatusInternalServerError)
		log.Print(err.Error())
		return
	}

	// Check if the uuid from the access token is the same as the authorID from the query
	// If not, return http.StatusUnauthorized
	// YOUR CODE HERE
	if uuid != authorID {
		http.Error(w, errors.New("uuid from access token does not match authorID from db query").Error(), http.StatusUnauthorized)
		log.Print("uuid from access token does not match authorID from db query")
		return
	}

	// Delete the post since by now we're authorized to do so
	_, err = DB.Exec("DELETE FROM posts WHERE postID = ?", postID)

	// Check for errors in executing the query
	// YOUR CODE HERE
	if err != nil {
		http.Error(w, errors.New("error in deleting the post").Error(), http.StatusInternalServerError)
		log.Print(err.Error())
		return
	}

	return
}

func getFeed(w http.ResponseWriter, r *http.Request) {
	// get the start index from the url paramaters
	// based on the previous functions, you should be familiar with how to do so
	// YOUR CODE HERE
	vars := mux.Vars(r)

	// convert startIndex to int
	// YOUR CODE HERE
	startIndex, err := strconv.Atoi(vars["startIndex"])

	// Check for errors in converting
	// If error, return http.StatusBadRequest
	// YOUR CODE HERE
	if err != nil {
		http.Error(w, errors.New("error in converting string to int").Error(), http.StatusBadRequest)
		log.Print(err.Error())
		return
	}

	// Get the userID from the access_token
	// You should now be familiar with how to do so
	// YOUR CODE HERE
	uuid := getUUID(w, r)

	// Obtain all of the posts where the authorID is *NOT* the current authorID
	// Sort chronologically
	// Always limit to 25 queries
	// Always start at an offset of startIndex
	posts, err := DB.Query("SELECT * FROM posts WHERE authorID <> ? ORDER BY postTime LIMIT ?, 25", uuid, startIndex)

	// Check for errors in executing the query
	// YOUR CODE HERE
	if err != nil {
		http.Error(w, errors.New("error querying all posts where authorID is not the current userID").Error(), http.StatusInternalServerError)
		log.Print(err.Error())
		return
	}

	var (
		content string
		postID string
		userid string
		postTime time.Time
	)

	// Put all the posts into an array of Max Size 25 and return all the filled spots
	// Almost exaclty like getPosts()
	// YOUR CODE HERE






	numPosts := 0
	postsArray := make([]Post, 25)
	for i := 0; i < 25 && posts.Next(); i++ {
		err = posts.Scan(&content, &postID, &userid, &postTime)
		if err != nil {
			http.Error(w, errors.New("error in scanning query contents into variables").Error(), http.StatusInternalServerError)
			log.Print(err.Error())
			return
		}
		postsArray[i] = Post{PostBody: content, PostID: postID, AuthorID: userid, PostTime: postTime, PostAuthor: userid}
		numPosts++
	}
	posts.Close()
	err = posts.Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err.Error())
	}
  json.NewEncoder(w).Encode(postsArray[:numPosts])
  return;
}
