package main

import (
	"fmt"
	"net/http"
	"time"

	"gostart/posts"
	"gostart/templates"

	"github.com/a-h/templ"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

var db *sqlx.DB

func main() {
	var p string = ":7000"

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		TemplRender(w, r, templates.StarterWelcome(""))
	})

	mux.HandleFunc("GET /error", func(w http.ResponseWriter, r *http.Request) {
		TemplRender(w, r, templates.StarterWelcome("Error!"))
	})

	mux.HandleFunc("POST /posts", func(w http.ResponseWriter, r *http.Request) {
		postID := r.FormValue("postID")
		http.Redirect(w, r, "/posts/"+postID, http.StatusSeeOther)
	})

	mux.HandleFunc("GET /posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("id")
		posts, err := posts.View(postID)
		if err != nil {
			http.Redirect(w, r, "/error", 500)
		}
		TemplRender(w, r, templates.Post("Posts", posts, postID))
	})

	mux.HandleFunc("GET /posts/{id}/new", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/posts/{id}", http.StatusSeeOther)
	})

	mux.HandleFunc("POST /posts/{id}/new", func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("id")
		post := posts.Post{UserID: 1, Content: r.FormValue("message"), CreatedAt: time.Now().String(), Name: r.FormValue("name"), PostID: postID}

		if vErr := posts.Validate(post); vErr != nil {
			fmt.Println("Error: ", vErr)
			posts, _ := posts.View(postID)
			TemplRender(w, r, templates.PartialPostNewError(posts, postID, vErr))
			return
		}

		if err := posts.Insert(post); err != nil {
			fmt.Println("Error inserting")
		}
		posts, err := posts.View(postID)
		if err != nil {
			http.Redirect(w, r, "/error", 500)
		}
		if hd := r.Header.Get("Hx-Request"); hd != "" {
			TemplRender(w, r, templates.PartialPostNewSuccess(posts, postID))
		}
	})

	mux.HandleFunc("GET /about", func(w http.ResponseWriter, r *http.Request) {
		TemplRender(w, r, templates.About())
	})

	mux.Handle("GET /static/", http.StripPrefix("/static", http.FileServer(http.Dir("./static"))))
	http.ListenAndServe(p, mux)
}

func TemplRender(w http.ResponseWriter, r *http.Request, c templ.Component) {
	posts.Connect()
	c.Render(r.Context(), w)
}
