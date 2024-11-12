package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"gorant/posts"
	"gorant/templates"

	"github.com/a-h/templ"

	_ "modernc.org/sqlite"
)

type User struct {
	Username string
	LoggedIn string
}

var ctx context.Context = context.Background()

func main() {
	// Instantiate a new API service.
	user := &User{Username: "", LoggedIn: "false"}

	service := NewAuthService(
		os.Getenv("STYTCH_PROJECT_ID"),
		os.Getenv("STYTCH_SECRET"),
	)

	// Register HTTP handlers.
	mux := http.NewServeMux()
	mux.Handle("/", service.RequireAuthentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := posts.ListPosts()
		if err != nil {
			fmt.Println("Error fetching posts")
		}

		TemplRender(w, r, templates.StarterWelcome("Welcome", p, user.Username, ""))
	})))

	mux.HandleFunc("GET /error", func(w http.ResponseWriter, r *http.Request) {
		TemplRender(w, r, templates.Error("Oops something went wrong."))
	})

	//--------------------------------------
	// Posts
	//--------------------------------------

	mux.HandleFunc("/posts", func(w http.ResponseWriter, r *http.Request) {
		postID := r.FormValue("post-id")
		exists := posts.VerifyPostID(postID)
		if !exists {
			http.Redirect(w, r, "/error", http.StatusSeeOther)
		}
		http.Redirect(w, r, "/posts/"+postID, http.StatusSeeOther)
	})

	mux.Handle("/posts/{postID}", service.CheckAuthentication(user, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("postID")
		post, comments, err := posts.GetPostComments(postID, user.Username)
		if err != nil {
			fmt.Println(err)
			TemplRender(w, r, templates.Error("Error!"))
			return
		}

		TemplRender(w, r, templates.Post("Posts", post, comments, postID, user.Username, user.LoggedIn))
	})))

	mux.HandleFunc("GET /posts/{postID}/new", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/posts/{postID}", http.StatusSeeOther)
	})

	mux.Handle("POST /posts/{postID}/new", service.CheckAuthentication(user, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("postID")

		if user.Username == "" {
			fmt.Println("Not authenticated")
			var comments []posts.JoinComment
			comments, err := posts.GetComments(postID, user.Username)
			if err != nil {
				fmt.Println(err)
				TemplRender(w, r, templates.Error("Error!"))
				return
			}
			TemplRender(w, r, templates.PartialPostNewErrorLogin(comments, postID, user.Username))
			return
		}

		if !posts.VerifyPostID(postID) {
			fmt.Println("Error verifying post exists")
			TemplRender(w, r, templates.Error("Error! Post doesn't exist!"))
			return
		}

		c := posts.Comment{
			UserID:    user.Username,
			Content:   r.FormValue("message"),
			CreatedAt: time.Now().String(),
			PostID:    postID,
		}

		if v := posts.Validate(c); v != nil {
			fmt.Println("Error: ", v)
			comments, err := posts.GetComments(postID, user.Username)
			if err != nil {
				fmt.Println("Error fetching posts")
				TemplRender(w, r, templates.Error("Oops, something went wrong."))
				return
			}
			TemplRender(w, r, templates.PartialPostNewError(comments, postID, user.Username, v))
			return
		}

		if err := posts.Insert(c); err != nil {
			fmt.Println("Error inserting")
		}
		comments, err := posts.GetComments(postID, user.Username)
		if err != nil {
			TemplRender(w, r, templates.Error("Oops, something went wrong."))
			return
		}
		if hd := r.Header.Get("Hx-Request"); hd != "" {
			TemplRender(w, r, templates.PartialPostNewSuccess(comments, postID, user.Username))
		}
	})))

	mux.Handle("POST /posts/{postID}/mood/edit/{newMood}", service.CheckAuthentication(user, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("postID")
		newMood := r.PathValue("newMood")

		if user.Username == "" {
			post, err := posts.GetPostInfo(postID, user.Username)
			if err != nil {
				fmt.Println(err)
			}
			TemplRender(w, r, templates.PartialEditMoodError(postID, post.Mood))
			return
		}

		if err := posts.EditMood(postID, newMood); err != nil {
			fmt.Println(err)
			return
		}

		post, err := posts.GetPostInfo(postID, user.Username)
		if err != nil {
			fmt.Println("Issue with getting post info: ", err)
		}

		TemplRender(w, r, templates.MoodMapper(postID, post.UserID, user.Username, post.Mood))
	})))

	mux.Handle("POST /posts/{postID}/comment/{commentID}/upvote", service.CheckAuthentication(user, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("postID")
		commentID := r.PathValue("commentID")

		if user.Username == "" {
			comments, err := posts.GetComments(postID, user.Username)
			if err != nil {
				fmt.Println("Error fetching posts: ", err)
			}
			TemplRender(w, r, templates.PartialPostVoteError(comments, postID, user.Username))
			return
		}

		var err error
		err = posts.UpVote(commentID, user.Username)
		if err != nil {
			fmt.Println("Error executing upvote", err)
		}

		var comments []posts.JoinComment
		comments, err = posts.GetComments(postID, user.Username)
		if err != nil {
			fmt.Println("Error fetching posts", err)
		}

		TemplRender(w, r, templates.PartialPostVote(comments, postID, user.Username))
	})))

	mux.Handle("POST /posts/{postID}/comment/{commentID}/delete", service.CheckAuthentication(user, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("postID")
		commentID := r.PathValue("commentID")

		if user.Username == "" {
			fmt.Println("I'm inside unauthenticated")
			comments, err := posts.GetComments(postID, user.Username)
			if err != nil {
				fmt.Println(err)
			}
			TemplRender(w, r, templates.PartialPostDeleteError(comments, postID, user.Username))
			return
		}

		if err := posts.Delete(commentID, user.Username); err != nil {
			fmt.Println("Error deleting comment: ", err)
			return
		}

		comments, err := posts.GetComments(postID, user.Username)
		if err != nil {
			fmt.Println("Error fetching posts", err)
		}
		TemplRender(w, r, templates.PartialPostDelete(comments, postID, user.Username))
	})))

	mux.Handle("GET /posts/{postID}/description/edit", service.CheckAuthentication(user, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("postID")
		if user.Username == "" {
			fmt.Println("Not authenticated, not allowed to edit description")
			return
		}

		post, err := posts.GetPostInfo(postID, user.Username)
		if err != nil {
			fmt.Println("Error fetching post info", err)
		}
		TemplRender(w, r, templates.PartialEditDescriptionInput(postID, post))
	})))

	mux.Handle("POST /posts/{postID}/description/edit", service.CheckAuthentication(user, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("postID")
		description := r.FormValue("post-description-input")

		err := posts.EditPostDescription(postID, description)
		if err != nil {
			fmt.Println(err)
			TemplRender(w, r, templates.Error("Something went wrong while editing the post!"))
		}

		post, err := posts.GetPostInfo(postID, user.Username)
		if err != nil {
			fmt.Println("Error fetching post info", err)
		}
		TemplRender(w, r, templates.PartialEditDescriptionResponse(postID, post))
	})))

	mux.HandleFunc("GET /about", func(w http.ResponseWriter, r *http.Request) {
		TemplRender(w, r, templates.About())
	})

	mux.HandleFunc("/admin/reset", func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("DEV_ENV") == "TRUE" {
			err := posts.ResetDB()
			if err != nil {
				w.Write([]byte("Reset failed, errored out"))
				return
			}

			t := time.Now().String()

			TemplRender(w, r, templates.Reset("", t))
		}
	})

	//--------------------------------------
	// Auth handles
	//--------------------------------------
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		TemplRender(w, r, templates.Login())
	})
	mux.HandleFunc("POST /login/sendlink", service.sendMagicLinkHandler)
	mux.HandleFunc("/authenticate", service.authenticateHandler)

	mux.Handle("GET /static/", http.StripPrefix("/static", http.FileServer(http.Dir("./static"))))

	var p string = os.Getenv("LISTEN_ADDR")
	wrappedMux := StatusLogger(ExcludeCompression(mux))
	http.ListenAndServe(p, wrappedMux)
}

func TemplRender(w http.ResponseWriter, r *http.Request, c templ.Component) {
	c.Render(r.Context(), w)
}
