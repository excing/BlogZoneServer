package main

import (
    "fmt"
    "io"
    "os"
    "time"
    "strconv"
    "regexp"
    "net/http"
    "crypto/md5"
    "database/sql"
    "encoding/json"
    "./config"
)

import _ "github.com/go-sql-driver/mysql"
import "github.com/gobuffalo/uuid"

type Blog struct {
    Id string `json:"id,omitempty"`
    Title string `json:"title,omitempty"`
    Content string `json:"content,omitempty"`
    ContentType string `json:"contentType,omitempty"`
    Channel string `json:"channel,omitempty"`
    EditTime int64 `json:"editTime,omitempty"`
    UpdateTime int64 `json:"updateTime,omitempty"`
    Token string `json:"token,omitempty"`
}

type BlogSlice struct {
    PageCount int `json:"pageCount"`
    BlogList []Blog `json:"list,omitempty"`
}

// 上传图像接口
func uploadFileHandle(w http.ResponseWriter, r *http.Request) {
    fmt.Println("method:", r.Method) //获取请求的方法

    os.Mkdir("uploaded", os.ModePerm)

    file, handler, err := r.FormFile("imageA")
    if err != nil {
        fmt.Println(err)
        errorHandle(err, w)
        return
    }

    defer file.Close()
    fmt.Println(handler.Header)
    f1, err := os.OpenFile("./uploaded/" + strconv.FormatInt(time.Now().Unix(), 10) + "_" +handler.Filename, os.O_WRONLY|os.O_CREATE, 0666)
    if err != nil {
        fmt.Println(err)
        errorHandle(err, w)
        return
    }
    defer f1.Close()
    io.Copy(f1, file)

    filename1 := f1.Name()

    // w.Write([]byte(filename1))
    wirteResponse(w, filename1)
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

    if (Name != "your_username") {
        wirteResponse(w, "false")
        return
    }

    UserPasswdMd5Bytes:= md5.Sum([]byte("your_password"))

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
    limit := 12;
    index, _ := strconv.Atoi(pageIndex)
    offset := (index - 1) * limit

    pageHandle(w, r, limit, offset)
}

func pageHandle(w http.ResponseWriter, r *http.Request, limit int, offset int) {
    fmt.Println(r.URL.Path, limit, offset)
    r.ParseForm()

    ClientToken := r.PostFormValue("token")

    if "" == ClientToken || ClientToken != CurrentToken {
        fmt.Println("该用户未登录")
    }

    var blogSlice BlogSlice
    blogSlice.PageCount = 0

    db, err := dbConn()

    if err != nil {
        fmt.Println(err)
        errorHandle(err, w)
        return
    } 

    sqlStr := "SELECT t1.blog_id, t1.blog_title, blog_edit_time  FROM s_blog t1 join (SELECT blog_id, max(id) as id from s_blog GROUP BY blog_id ) t2 ON t1.id = t2.id ORDER BY t1.sql_update_time DESC LIMIT ? OFFSET ?;"

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

        if len(title) <= 0 {
            continue
        }

        var blog Blog
        blog.Id = id;
        blog.Title = title
        blog.EditTime = editTime

        blogSlice.BlogList = append(blogSlice.BlogList, blog)

        fmt.Println(id, title)
    }

    sqlStr = "SELECT count(t1.blog_id) FROM s_blog t1 join (SELECT blog_id, max(id) as id from s_blog GROUP BY blog_id ) t2 ON t1.id = t2.id ORDER BY t1.sql_update_time DESC;"

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

    sqlStr = "INSERT INTO s_blog(blog_title, blog_content, blog_content_type, blog_channel, blog_edit_time, sql_update_time, blog_id) VALUES (?, ?, ?, ?, ?, ?, ?)"

    title := blog.Title
    content := blog.Content
    contentType := blog.ContentType
    channel := blog.Channel
    editTime := blog.EditTime
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

    result, err := db.Exec(sqlStr, title, content, contentType, channel, editTimeInt, updateTime, id)

    defer db.Close()

    if err != nil {
        fmt.Println(err)
        errorHandle(err, w)
        return
    } 

    fmt.Println(result)

    // w.Write([]byte(id))
    wirteResponse(w, id)
}

func viewHandle(w http.ResponseWriter, r *http.Request, blogId string) {
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

    sqlStr := "SELECT blog_title, blog_content, blog_content_type, blog_channel, blog_edit_time FROM s_blog WHERE blog_id=? ORDER BY sql_update_time DESC LIMIT 1;"

    result, err := db.Query(sqlStr, blogId)
    if err != nil {
        fmt.Println(err)
        errorHandle(err, w)
        return
    }

    defer db.Close()

    var blog Blog

    for result.Next() {
        var title, content, content_type, channel string
        var edit_time int64
        err = result.Scan(&title, &content, &content_type, &channel, &edit_time)
        if err != nil {
            fmt.Println(err)
            errorHandle(err, w)
            return
        }

        blog.Id = blogId;
        blog.Title = title
        blog.Content = content
        blog.ContentType = content_type
        blog.Channel = channel
        blog.EditTime = edit_time

        fmt.Println(title, content, content_type)
    }

    blogJson, err := json.Marshal(blog)
    if err != nil {
        fmt.Println(err)
        errorHandle(err, w)
        return
    }

    fmt.Println(blog)

    // w.Write([]byte("ok"))
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
        blog.Id = blogId;
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

// 统一错误输出接口
func errorHandle(err error, w http.ResponseWriter) {
    if  err != nil {
        w.Write([]byte(err.Error()))
    }
}

var validPath = regexp.MustCompile("^/(view|history|list)/([a-zA-Z0-9-]+)$")

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
    w.Header().Add("Content-Type", "Application/json") //header的类型
    w.Write([]byte(body))
}

func dbConn() (db *sql.DB, err error) {
    dbDriver := "mysql"
    dbUser := "root"
    dbPass := "123456"
    dbName := "myblog"
    return sql.Open(dbDriver, dbUser+":"+dbPass+"@/"+dbName)
}

func main() {
    fmt.Println(config.GetUsername())

    http.HandleFunc("/uploadFile", uploadFileHandle) // 上传
    http.HandleFunc("/", indexHandle)
    http.HandleFunc("/login", loginHandle)
    http.HandleFunc("/online", onlineHandle)
    http.HandleFunc("/edit", editHandle)
    http.HandleFunc("/list/", makeHandler(listHandle))
    http.HandleFunc("/view/", makeHandler(viewHandle))
    http.HandleFunc("/history/", makeHandler(historyHandle))
    err := http.ListenAndServe(":8080", nil)
    fmt.Println(err)
}