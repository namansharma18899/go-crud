package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/thedevsaddam/renderer"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var rnd *renderer.Render

var db *mgo.Database

const (
	hostName       string = "localhost:27017"
	dbName         string = "demo_table"
	collectionName string = "todo"
	port           string = ":9000"
)

type (
	todoModel struct {
		ID        bson.ObjectId `bson:"_id,omitempty"`
		Title     string        `bson:"title"`
		Completed bool          `bson:"completed"`
		CreatedAt time.Time     `bson:"createdAt"`
	}

	todo struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		Completed bool      `json:"completed"`
		CreatedAt time.Time `json:"createdAt"`
	}
)

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	rnd = renderer.New()
	sess, err := mgo.Dial(hostName)
	checkErr(err)
	sess.SetMode(mgo.Monotonic, true)
}

func todoHandler() http.Handler {
	rg := chi.NewRouter()
	rg.Group(func(r chi.Router) {
		r.Get("/", fetchTodos)
		r.Post("/", createTodo)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})
	return rg
}

func homeHandler(w http.ResponseWriter, req *http.Request) {
	err := rnd.Template(w, http.StatusOK, []string{"/static/home.tpl"}, nil)
	checkErr(err)
}

func validateTodoObject(t todo) bool {
	if t.Title == " " {
		// rnd.JSON(w, http.StatusBadRequest)
		return false
	}
	return true
}

func createTodo(w http.ResponseWriter, req *http.Request) {
	var t todo
	if err := json.NewDecoder(req.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "failed to fetch todo",
			"error":   err,
		})
		return
	}
	// we need to validate the data and persist it in db
	if validateTodoObject(t) == false {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Validation Error for TODO",
			"error":   "",
		})
		return
	}

	tm := todoModel{
		ID:        bson.NewObjectId(),
		Title:     t.Title,
		Completed: t.Completed,
		CreatedAt: t.CreatedAt,
	}

	if err := db.C(collectionName).Insert(&tm); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Validation Error for TODO",
			"error":   err,
		})
		return
	}

	rnd.JSON(w, http.StatusCreated, renderer.M{
		"message": "todo created successfully",
		"todo_id": tm.ID.Hex()})
}

func updateTodo(w http.ResponseWriter, req *http.Request) {
	id := strings.TrimSpace(chi.URLParam(req, "id"))
	if !bson.IsObjectIdHex(id) {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Invalid todo id",
			"error":   "Object Id not present",
		})
		return
	}
	// fetch tempTodo
	var tempTodo todo

	if err := json.NewDecoder(req.Body).Decode(&tempTodo); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}

	if validateTodoObject(tempTodo) == false {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Invalid todo id",
			"error":   "validation failed",
		})
		return
	}

	if err := db.C(collectionName).Update(
		bson.M{"_id": bson.ObjectIdHex(id)},
		bson.M{"title": tempTodo.Title},
	); err != nil {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Failed to update todo",
			"error":   err,
		})
		return
	}

	// tempTodo := todo{}
	// if err := db.C(collectionName).FindId(id).All(&tempTodo); err != nil {
	// 	rnd.JSON(w, http.StatusProcessing, renderer.M{
	// 		"message": "Failed to delete todo",
	// 		"error":   err})
	// 	return
	// }
	// td := todoModel{
	// 	ID:        bson.NewObjectId(),
	// 	Title:     tempTodo.Title,
	// 	Completed: tempTodo.Completed,
	// 	CreatedAt: tempTodo.CreatedAt,
	// }
	// if err := db.C(collectionName).UpdateId(td.ID, &td); err != nil {
	// 	rnd.JSON(w, http.StatusProcessing, renderer.M{
	// 		"message": "Failed to delete todo",
	// 		"error":   err})
	// 	return
	// }
}

func deleteTodo(w http.ResponseWriter, req *http.Request) {
	id := strings.TrimSpace(chi.URLParam(req, "id"))

	if !bson.IsObjectIdHex(id) {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Invalid todo id",
			"error":   "Object Id not present",
		})
		return
	}
	if err := db.C(collectionName).RemoveId(id); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to delete todo",
			"error":   err,
		})
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "todo deleted successfully",
	})
}

func fetchTodos(w http.ResponseWriter, req *http.Request) {
	todos := []todo{}
	if err := db.C(collectionName).Find(bson.M{}).All(&todos); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to fetch todo",
			"error":   err,
		})
		return
	}
	todoList := []todo{}

	for _, t := range todos {
		todoList = append(todoList, todo{
			ID:        t.ID.Hex(),
			Title:     t.Title,
			Completed: t.Completed,
			CreatedAt: t.CreatedAt,
		})
	}
	rnd.JSON(w, http.StatusOK, renderer.M{
		"data": todoList,
	})
}

func main() {
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homeHandler)
	r.Mount("/todo", todoHandler())

	srv := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Println("Listening on Port: ", port)
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("listen..:%s\n", err)
		}
	}()

	<-stopChan // Listen for the channel
	log.Println("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	srv.Shutdown(ctx)
	defer cancel()
	log.Println("server gracefully shutting down!")
}
