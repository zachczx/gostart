package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"gorant/database"
	"gorant/posts"
	"gorant/templates"
	"gorant/users"

	"github.com/a-h/templ"

	_ "modernc.org/sqlite"
)

type User struct {
	Username string
}

var ctx context.Context = context.Background()

func main() {
	service := NewAuthService(
		os.Getenv("STYTCH_PROJECT_ID"),
		os.Getenv("STYTCH_SECRET"),
	)

	mux := http.NewServeMux()
	mux.Handle("/", service.CheckAuthentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.Context().Value("currentUser"))
		p, err := posts.ListPosts()
		if err != nil {
			fmt.Println("Error fetching posts", err)
		}

		TemplRender(w, r, templates.StarterWelcome("Welcome", p))
	})))

	mux.HandleFunc("GET /error", func(w http.ResponseWriter, r *http.Request) {
		TemplRender(w, r, templates.Error("Oops something went wrong."))
	})

	mux.Handle("/posts", service.CheckAuthentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.FormValue("post-id")

		if v := posts.ValidatePost(postID); v != nil {
			fmt.Println(v)

			if r.Header.Get("Hx-request") != "" {
				TemplRender(w, r, templates.PartialStarterWelcomeError())
				return
			}
			http.Redirect(w, r, "/error", http.StatusSeeOther)
			return
		}

		exists := posts.VerifyPostID(postID)
		if exists {
			http.Redirect(w, r, "/posts/"+postID, http.StatusSeeOther)
		}

		if err := posts.NewPost(postID, r.Context().Value("currentUser").(string)); err != nil {
			fmt.Println(err)
			http.Redirect(w, r, "/login?r=new", http.StatusSeeOther)
		}

		http.Redirect(w, r, "/posts/"+postID, http.StatusSeeOther)
	})))

	mux.Handle("/posts/{postID}", service.CheckAuthentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("postID")
		post, comments, err := posts.GetPostComments(postID, r.Context().Value("currentUser").(string))
		if err != nil {
			fmt.Println(err)
			TemplRender(w, r, templates.Error("Error!"))
			return
		}

		TemplRender(w, r, templates.Post("Posts", post, comments, postID))
	})))

	mux.HandleFunc("GET /posts/{postID}/new", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/posts/{postID}", http.StatusSeeOther)
	})

	mux.Handle("POST /posts/{postID}/new", service.CheckAuthentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("postID")

		if r.Context().Value("currentUser").(string) == "" {
			fmt.Println("Not authenticated")
			var comments []posts.JoinComment
			comments, err := posts.GetComments(postID, r.Context().Value("currentUser").(string))
			if err != nil {
				fmt.Println(err)
				TemplRender(w, r, templates.Error("Error!"))
				return
			}
			TemplRender(w, r, templates.PartialPostNewErrorLogin(comments, postID))
			return
		}

		if !posts.VerifyPostID(postID) {
			fmt.Println("Error verifying post exists")
			TemplRender(w, r, templates.Error("Error! Post doesn't exist!"))
			return
		}

		c := posts.Comment{
			UserID:    r.Context().Value("currentUser").(string),
			Content:   r.FormValue("message"),
			CreatedAt: time.Now().String(),
			PostID:    postID,
		}

		if v := posts.Validate(c); v != nil {
			fmt.Println("Error: ", v)
			comments, err := posts.GetComments(postID, r.Context().Value("currentUser").(string))
			if err != nil {
				fmt.Println("Error fetching posts")
				TemplRender(w, r, templates.Error("Oops, something went wrong."))
				return
			}
			TemplRender(w, r, templates.PartialPostNewError(comments, postID, v))
			return
		}

		var insertedID string
		insertedID, err := posts.Insert(c)
		if err != nil {
			fmt.Println("Error inserting: ", err)
		}
		comments, err := posts.GetComments(postID, r.Context().Value("currentUser").(string))
		if err != nil {
			TemplRender(w, r, templates.Error("Oops, something went wrong."))
			return
		}
		if hd := r.Header.Get("Hx-Request"); hd != "" {
			TemplRender(w, r, templates.PartialPostNewSuccess(comments, postID, insertedID))
		}
	})))

	mux.Handle("POST /posts/{postID}/mood/edit/{newMood}", service.CheckAuthentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("postID")
		newMood := r.PathValue("newMood")

		if r.Context().Value("currentUser").(string) == "" {
			post, err := posts.GetPostInfo(postID, r.Context().Value("currentUser").(string))
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

		post, err := posts.GetPostInfo(postID, r.Context().Value("currentUser").(string))
		if err != nil {
			fmt.Println("Issue with getting post info: ", err)
		}

		TemplRender(w, r, templates.MoodMapper(postID, post.UserID, post.Mood))
	})))

	mux.Handle("POST /posts/{postID}/comment/{commentID}/upvote", service.CheckAuthentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("postID")
		commentID := r.PathValue("commentID")

		if r.Context().Value("currentUser").(string) == "" {
			comments, err := posts.GetComments(postID, r.Context().Value("currentUser").(string))
			if err != nil {
				fmt.Println("Error fetching posts: ", err)
			}
			TemplRender(w, r, templates.PartialPostVoteError(comments, postID))
			return
		}

		var err error
		err = posts.UpVote(commentID, r.Context().Value("currentUser").(string))
		if err != nil {
			fmt.Println("Error executing upvote", err)
		}

		var comments []posts.JoinComment
		comments, err = posts.GetComments(postID, r.Context().Value("currentUser").(string))
		if err != nil {
			fmt.Println("Error fetching posts", err)
		}

		TemplRender(w, r, templates.PartialPostVote(comments, postID, commentID))
	})))

	mux.Handle("POST /posts/{postID}/comment/{commentID}/delete", service.CheckAuthentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("postID")
		commentID := r.PathValue("commentID")

		if r.Context().Value("currentUser").(string) == "" {
			fmt.Println("I'm inside unauthenticated")
			comments, err := posts.GetComments(postID, r.Context().Value("currentUser").(string))
			if err != nil {
				fmt.Println(err)
			}
			TemplRender(w, r, templates.PartialPostDeleteError(comments, postID))
			return
		}

		if err := posts.Delete(commentID, r.Context().Value("currentUser").(string)); err != nil {
			fmt.Println("Error deleting comment: ", err)
			return
		}

		comments, err := posts.GetComments(postID, r.Context().Value("currentUser").(string))
		if err != nil {
			fmt.Println("Error fetching posts", err)
		}
		TemplRender(w, r, templates.PartialPostDelete(comments, postID))
	})))

	mux.Handle("POST /posts/{postID}/description/edit", service.CheckAuthentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postID := r.PathValue("postID")
		description := r.FormValue("post-description-input")

		err := posts.EditPostDescription(postID, description)
		if err != nil {
			fmt.Println(err)
			TemplRender(w, r, templates.Error("Something went wrong while editing the post!"))
		}

		post, err := posts.GetPostInfo(postID, r.Context().Value("currentUser").(string))
		if err != nil {
			fmt.Println("Error fetching post info", err)
		}
		TemplRender(w, r, templates.PartialEditDescriptionResponse(postID, post))
	})))

	mux.HandleFunc("GET /about", func(w http.ResponseWriter, r *http.Request) {
		TemplRender(w, r, templates.About())
	})

	mux.HandleFunc("GET /admin/reset", func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("DEV_ENV") == "TRUE" {
			err := database.Reset()
			if err != nil {
				fmt.Println(err)
				w.Write([]byte("Reset failed, errored out"))
				return
			}

			t := time.Now().String()

			TemplRender(w, r, templates.Reset("", t))
		} else {
			w.Write([]byte("Not allowed!"))
		}
	})

	mux.Handle("/settings", service.CheckAuthentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ref := r.URL.Query().Get("r")
		s, err := users.GetSettings(r.Context().Value("currentUser").(string))
		if err != nil {
			fmt.Println("Error fetching settings: ", err)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}

		switch ref {
		case "firstlogin":
			TemplRender(w, r, templates.SettingsFirstLogin(s))
			return
		}
		TemplRender(w, r, templates.Settings(s))
	})))

	mux.Handle("POST /settings/edit", service.RequireAuthentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := users.Settings{
			PreferredName: r.FormValue("preferred-name"),
			ContactMe:     r.FormValue("contact-me"),
		}

		if err := users.Validate(f); err != nil {
			fmt.Println("Error: ", err)
			s, err := users.GetSettings(r.Context().Value("currentUser").(string))
			if err != nil {
				fmt.Println("Error fetching settings: ", err)
				http.Redirect(w, r, "/error", http.StatusSeeOther)
			}
			TemplRender(w, r, templates.PartialSettingsEditError(s))
			return
		}

		if err := users.SaveSettings(r.Context().Value("currentUser").(string), f); err != nil {
			fmt.Println("Error saving: ", err)
			http.Redirect(w, r, "/error", http.StatusSeeOther)
		}

		s, err := users.GetSettings(r.Context().Value("currentUser").(string))
		if err != nil {
			fmt.Println("Error fetching settings: ", err)
			http.Redirect(w, r, "/error", http.StatusSeeOther)
		}
		fmt.Println(s)
		TemplRender(w, r, templates.PartialSettingsEditSuccess(s))
	})))

	//--------------------------------------
	// Auth handles
	//--------------------------------------
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		ref := r.URL.Query().Get("r")

		switch ref {
		case "new":
			TemplRender(w, r, templates.Login("error", "You need to login before you can create a new post"))
			return
		}
		TemplRender(w, r, templates.Login("", ""))
	})

	mux.HandleFunc("POST /login/sendlink", service.sendMagicLinkHandler)

	mux.Handle("/authenticate", service.authenticateHandler(ctx))

	mux.Handle("/logout", service.logout(ctx, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		TemplRender(w, r, templates.LoggedOut())
	})))

	mux.Handle("GET /static/", http.StripPrefix("/static", http.FileServer(http.Dir("./static"))))

	var p string = os.Getenv("LISTEN_ADDR")
	wrappedMux := StatusLogger(ExcludeCompression(mux))
	http.ListenAndServe(p, wrappedMux)
}

func TemplRender(w http.ResponseWriter, r *http.Request, c templ.Component) {
	c.Render(r.Context(), w)
}
