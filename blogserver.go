package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"./config"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gobuffalo/uuid"
)

const PWD_WORDS = "0123456789abcdefghijklmnopqrstuvwxyz"

type Blog struct {
	Id          string `json:"id,omitempty"`
	Title       string `json:"title,omitempty"`
	Content     string `json:"content,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Channel     string `json:"channel,omitempty"`
	EditTime    int64  `json:"editTime,omitempty"`
	CreateTime  int64  `json:"createTime,omitempty"`
	Token       string `json:"token,omitempty"`
	Mode        int64  `json:"mode,omitempty"`
}

type BlogSlice struct {
	PageCount int    `json:"pageCount"`
	BlogList  []Blog `json:"list,omitempty"`
}

var CurrentToken string

func loginHandle(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	Name := r.PostFormValue("name")
	PasswdMd5Str := r.PostFormValue("passwd")
	ClientToken := r.PostFormValue("token")

	// fmt.Println("1", CurrentToken, "-", ClientToken)

	if "" != ClientToken && ClientToken == CurrentToken {
		NewToken := uuid.Must(uuid.NewV4()).String()

		CurrentToken = NewToken

		wirteResponse(w, NewToken)

		return
	}

	if Name != config.GetUsername() {
		wirteResponse(w, "false")
		return
	}

	UserPasswdMd5Bytes := md5.Sum([]byte(config.GetPassword()))

	// fmt.Println("2", PasswdMd5Str, fmt.Sprintf("%x", UserPasswdMd5Bytes))

	if PasswdMd5Str == fmt.Sprintf("%x", UserPasswdMd5Bytes) {
		// fmt.Println("3. Login Success")

		NewToken := uuid.Must(uuid.NewV4()).String()

		CurrentToken = NewToken

		// fmt.Println("4", CurrentToken, NewToken)

		wirteResponse(w, NewToken)
	} else {
		wirteResponse(w, "false")
	}
}

func logoutHandle(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	ClientToken := r.PostFormValue("token")

	if "" != ClientToken && ClientToken == CurrentToken {
		NewToken := uuid.Must(uuid.NewV4()).String()

		CurrentToken = NewToken

		wirteResponse(w, "true")
	} else {
		wirteResponse(w, "false")
	}
}

func onlineHandle(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	ClientToken := r.PostFormValue("token")

	if "" != ClientToken && ClientToken == CurrentToken {
		wirteResponse(w, "true")
	} else {
		wirteResponse(w, "false")
	}
}

func indexHandle(w http.ResponseWriter, r *http.Request) {
	pageHandle(w, r, 5, 0)
}

func listHandle(w http.ResponseWriter, r *http.Request, pageIndex string) {
	limit := 12
	index, _ := strconv.Atoi(pageIndex)
	offset := (index - 1) * limit

	pageHandle(w, r, limit, offset)
}

func pageHandle(w http.ResponseWriter, r *http.Request, limit int, offset int) {
	fmt.Println(r.URL.Path, limit, offset)
	r.ParseForm()

	ClientToken := r.PostFormValue("token")

	isLogin := !("" == ClientToken || ClientToken != CurrentToken)

	var blogSlice BlogSlice
	blogSlice.PageCount = 0

	db, err := dbConn()

	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	var sqlStr string

	if isLogin {
		fmt.Println("该用户已登录")
		sqlStr = "SELECT t1.blog_id, t1.blog_title, blog_edit_time FROM s_blog t1 JOIN (SELECT blog_id, MAX(id) AS id FROM s_blog GROUP BY blog_id ) t2 ON t1.id = t2.id WHERE t1.blog_title <> '' ORDER BY t1.sql_update_time DESC LIMIT ? OFFSET ?"
	} else {
		fmt.Println("该用户未登录")
		sqlStr = "SELECT t1.blog_id, t1.blog_title, blog_edit_time FROM s_blog t1 JOIN (SELECT blog_id, MAX(id) AS id FROM s_blog GROUP BY blog_id ) t2 ON t1.id = t2.id WHERE t1.blog_id NOT IN (SELECT res_id AS id FROM s_mode WHERE res_mode IN (1, 3, 4)) AND t1.blog_title <> '' ORDER BY t1.sql_update_time DESC LIMIT ? OFFSET ?"
	}

	result, err := db.Query(sqlStr, limit, offset)

	defer db.Close()

	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	for result.Next() {
		var id, title string
		var editTime int64
		err = result.Scan(&id, &title, &editTime)
		if err != nil {
			fmt.Println(err)
			errorHandle(err, w)
			return
		}

		// if len(title) <= 0 {
		// 	continue
		// }

		var blog Blog
		blog.Id = id
		blog.Title = title
		blog.EditTime = editTime

		blogSlice.BlogList = append(blogSlice.BlogList, blog)

		fmt.Println(id, title)
	}

	if isLogin {
		sqlStr = "SELECT count(t1.blog_id) FROM s_blog t1 join (SELECT blog_id, max(id) as id from s_blog GROUP BY blog_id ) t2 ON t1.id = t2.id WHERE t1.blog_title <> '' ORDER BY t1.sql_update_time DESC"
	} else {
		sqlStr = "SELECT count(t1.blog_id) FROM s_blog t1 JOIN (SELECT blog_id, MAX(id) AS id FROM s_blog GROUP BY blog_id ) t2 ON t1.id = t2.id WHERE t1.blog_id NOT IN (SELECT res_id AS id FROM s_mode WHERE res_mode IN (1, 3, 4)) AND t1.blog_title <> ''"
	}

	result, err = db.Query(sqlStr)

	defer db.Close()

	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	for result.Next() {
		var itemCount int
		err = result.Scan(&itemCount)
		if err != nil {
			fmt.Println(err)
			errorHandle(err, w)
			return
		}

		blogSlice.PageCount = (itemCount + limit - 1) / limit
	}

	blogListJson, err := json.Marshal(blogSlice)
	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	fmt.Println(blogSlice.BlogList)

	// w.Write([]byte("ok"))
	wirteResponse(w, string(blogListJson))
}

func editHandle(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var blog Blog
	err := decoder.Decode(&blog)

	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	if blog.Token != CurrentToken {
		fmt.Println("未登录")
		wirteResponse(w, "false")
		return
	}

	id := blog.Id

	sqlStr := ""

	if len(id) == 0 {
		id = uuid.Must(uuid.NewV4()).String()
	}

	title := blog.Title
	content := blog.Content
	contentType := blog.ContentType
	channel := blog.Channel
	editTime := blog.EditTime
	mode := blog.Mode
	updateTime := time.Now().UnixNano() / int64(time.Millisecond)

	var editTimeInt int64

	if editTime == 0 {
		editTimeInt = updateTime
	} else {
		editTimeInt = editTime
	}

	db, err := dbConn()

	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	sqlStr = "select res_mode, res_pwd from s_mode where res_id=?"

	result, err := db.Query(sqlStr, id)
	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	defer db.Close()

	var res_pwd sql.NullString
	var res_mode sql.NullInt64

	for result.Next() {
		err := result.Scan(&res_mode, &res_pwd)
		if err != nil {
			fmt.Println(err)
			errorHandle(err, w)
			return
		}
	}

	// 是否需要设置博文权限
	// 如果从来没有设置该博文权限，则设置
	// 如果该博文原权限与新权限不一致，则设置
	// 如果该博文原权限与新权限一致，但需要口令查看且无口令，则设置
	var hasSetMode bool

	if res_mode.Valid {
		if mode == res_mode.Int64 {
			hasSetMode = res_pwd.Valid
		} else {
			hasSetMode = false
		}
	} else {
		hasSetMode = false
	}

	if !hasSetMode {

		sqlStr = "DELETE FROM s_mode where res_id=?"

		_, _ = db.Exec(sqlStr, id)

		defer db.Close()

		var ResPwd string

		if 2 == mode || 4 == mode {
			ResPwd = genResPwd()
		}

		sqlStr = "INSERT INTO s_mode(res_id, res_mode, res_pwd) value (?, ?, ?)"

		_, err = db.Exec(sqlStr, id, mode, ResPwd)

		defer db.Close()

		if err != nil {
			fmt.Println(err)
			errorHandle(err, w)
			return
		}
	}

	sqlStr = "INSERT INTO s_blog(blog_title, blog_content, blog_content_type, blog_channel, blog_edit_time, sql_update_time, blog_id) VALUES (?, ?, ?, ?, ?, ?, ?)"

	_, err = db.Exec(sqlStr, title, content, contentType, channel, editTimeInt, updateTime, id)

	defer db.Close()

	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	// w.Write([]byte(id))
	wirteResponse(w, id)
}

func deleteHandle(w http.ResponseWriter, r *http.Request, blogId string) {
	fmt.Println(r.URL.Path)
	r.ParseForm()

	ClientToken := r.PostFormValue("token")

	isLogin := "" != ClientToken && ClientToken == CurrentToken

	if !isLogin {
		fmt.Println("未登录")
		wirteResponse(w, "false")
		return
	}

	db, err := dbConn()

	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	defer db.Close()

	sqlStr := "DELETE FROM s_blog WHERE blog_id=?"

	_, err = db.Query(sqlStr, blogId)
	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	wirteResponse(w, "ok")
}

func viewHandle(w http.ResponseWriter, r *http.Request, blogId string) {
	fmt.Println(r.URL.Path)
	r.ParseForm()

	ClientToken := r.PostFormValue("token")

	isLogin := "" != ClientToken && ClientToken == CurrentToken

	if !isLogin {
		fmt.Println("该用户未登录")
	}

	db, err := dbConn()

	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	sqlStr := "select res_mode, res_pwd from s_mode where res_id=?"

	result, err := db.Query(sqlStr, blogId)
	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	defer db.Close()

	var blog Blog
	var mode sql.NullInt64
	var pwd sql.NullString

	for result.Next() {
		err = result.Scan(&mode, &pwd)
		if err != nil {
			fmt.Println(err)
			errorHandle(err, w)
			return
		}

		blog.Id = blogId

		if mode.Valid {
			blog.Mode = mode.Int64
		} else {
			blog.Mode = 0
		}
	}

	// 是否有权访问该文章
	var isVisit bool

	if !isLogin {
		if 2 == blog.Mode || 4 == blog.Mode {
			pwdMd5Str := r.PostFormValue("pwd")
			blogPwdMd5Bytes := md5.Sum([]byte(pwd.String))

			isVisit = pwdMd5Str == fmt.Sprintf("%x", blogPwdMd5Bytes)
		} else if 3 == blog.Mode {
			isVisit = false
		} else {
			isVisit = true
		}
	} else {
		isVisit = true
	}

	if !isVisit {
		blogJson, err := json.Marshal(blog)
		if err != nil {
			fmt.Println(err)
			errorHandle(err, w)
			return
		}

		wirteResponse(w, string(blogJson))
		return
	}

	sqlStr = "SELECT a.blog_title, a.blog_content, a.blog_content_type, a.blog_channel, a.blog_edit_time, (SELECT sql_update_time FROM s_blog a WHERE a.blog_id=? ORDER BY a.sql_update_time LIMIT 1) AS blog_create_time FROM s_blog a WHERE a.blog_id=? ORDER BY a.sql_update_time DESC LIMIT 1;"

	result, err = db.Query(sqlStr, blogId, blogId)
	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	defer db.Close()

	for result.Next() {
		var title, content, content_type, channel string
		var edit_time, create_time int64
		err = result.Scan(&title, &content, &content_type, &channel, &edit_time, &create_time)
		if err != nil {
			fmt.Println(err)
			errorHandle(err, w)
			return
		}

		blog.Id = blogId
		blog.Title = title
		blog.Content = content
		blog.ContentType = content_type
		blog.Channel = channel
		blog.EditTime = edit_time
		blog.CreateTime = create_time

		fmt.Println(title, content, content_type)
	}

	blogJson, err := json.Marshal(blog)
	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	wirteResponse(w, string(blogJson))
}

func historyHandle(w http.ResponseWriter, r *http.Request, blogId string) {
	fmt.Println(r.URL.Path)
	r.ParseForm()

	ClientToken := r.PostFormValue("token")

	if "" == ClientToken || ClientToken != CurrentToken {
		fmt.Println("该用户未登录")
	}

	db, err := dbConn()

	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	sqlStr := "SELECT blog_title, blog_content, blog_content_type FROM s_blog WHERE blog_id=? ORDER BY sql_update_time DESC;"

	result, err := db.Query(sqlStr, blogId)
	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	defer db.Close()

	var blogSlice BlogSlice

	for result.Next() {
		var title, content, content_type string
		err = result.Scan(&title, &content, &content_type)
		if err != nil {
			fmt.Println(err)
			errorHandle(err, w)
			return
		}

		var blog Blog
		blog.Id = blogId
		blog.Title = title
		blog.Content = content
		blog.ContentType = content_type

		blogSlice.BlogList = append(blogSlice.BlogList, blog)

		fmt.Println(title, content, content_type)
	}

	blogListJson, err := json.Marshal(blogSlice)
	if err != nil {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	fmt.Println(blogSlice.BlogList)

	// w.Write([]byte("ok"))
	wirteResponse(w, string(blogListJson))
}

func getBlogPwdHandle(w http.ResponseWriter, r *http.Request, BlogId string) {
	r.ParseForm()

	ClientToken := r.PostFormValue("token")

	if "" == ClientToken || ClientToken != CurrentToken {
		fmt.Println("未登录")
		wirteResponse(w, "false")
		return
	}

	sqlStr := "select res_pwd from s_mode where res_id=?"

	db, err := dbConn()

	if nil != err {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	result, err := db.Query(sqlStr, BlogId)

	if nil != err {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	var pwd sql.NullString

	for result.Next() {
		err := result.Scan(&pwd)

		if nil != err {
			fmt.Println(err)
			errorHandle(err, w)
			return
		}
	}

	if pwd.Valid {
		wirteResponse(w, pwd.String)
	} else {
		wirteResponse(w, "")
	}
}

func updateBlogPwdHandle(w http.ResponseWriter, r *http.Request, BlogId string) {
	r.ParseForm()

	ClientToken := r.PostFormValue("token")

	if "" == ClientToken || ClientToken != CurrentToken {
		fmt.Println("未登录")
		wirteResponse(w, "false")
		return
	}

	ResPwd := r.PostFormValue("pwd")

	sqlStr := "update s_mode set res_pwd=? where res_id=?"

	db, err := dbConn()

	if nil != err {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	fmt.Println(sqlStr, ResPwd, BlogId)

	_, err = db.Exec(sqlStr, ResPwd, BlogId)

	if nil != err {
		fmt.Println(err)
		errorHandle(err, w)
		return
	}

	wirteResponse(w, "true")
}

func genResPwd() string {
	rand.Seed(time.Now().UnixNano())

	_0 := rand.Intn(36)
	_1 := rand.Intn(36)
	_2 := rand.Intn(36)
	_3 := rand.Intn(36)

	ResPwd := PWD_WORDS[_0:_0+1] + PWD_WORDS[_1:_1+1] + PWD_WORDS[_2:_2+1] + PWD_WORDS[_3:_3+1]

	fmt.Println(ResPwd)

	return ResPwd
}

// 统一错误输出接口
func errorHandle(err error, w http.ResponseWriter) {
	if err != nil {
		w.Write([]byte(err.Error()))
	}
}

var validPath = regexp.MustCompile("^/(view|history|list|getBlogPwd|updateBlogPwd)/([a-zA-Z0-9-]+)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func wirteResponse(w http.ResponseWriter, body string) {
	w.Header().Set("Access-Control-Allow-Origin", "*")             //允许访问所有域
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type") //header的类型
	w.Header().Add("Content-Type", "Application/json")             //header的类型
	w.Write([]byte(body))
}

func dbConn() (db *sql.DB, err error) {
	dbDriver := config.GetDBDriver()
	dbUser := config.GetDBUser()
	dbPass := config.GetDBPassword()
	dbName := config.GetDBName()
	return sql.Open(dbDriver, dbUser+":"+dbPass+"@/"+dbName)
}

func main() {
	http.HandleFunc("/", indexHandle)
	http.HandleFunc("/login", loginHandle)
	http.HandleFunc("/logout", logoutHandle)
	http.HandleFunc("/online", onlineHandle)
	http.HandleFunc("/edit", editHandle)
	http.HandleFunc("/list/", makeHandler(listHandle))
	http.HandleFunc("/view/", makeHandler(viewHandle))
	http.HandleFunc("/delete/", makeHandler(deleteHandle))
	http.HandleFunc("/history/", makeHandler(historyHandle))
	http.HandleFunc("/getBlogPwd/", makeHandler(getBlogPwdHandle))
	http.HandleFunc("/updateBlogPwd/", makeHandler(updateBlogPwdHandle))
	err := http.ListenAndServe(":8080", nil)
	fmt.Println(err)
}
