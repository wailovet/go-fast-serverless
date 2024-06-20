package easyhttp

import (
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type Request struct {
	GET           map[string]string
	POST          map[string]string
	REQUEST       map[string]string
	COOKIE        map[string]string
	SESSION       map[string]string
	HEADER        map[string]string
	BODY          string
	FILES         map[string][]*multipart.FileHeader
	FILE          *multipart.FileHeader
	OriginRequest *http.Request
	OriginValues  url.Values
}

func (r *Request) SyncGetData(request *http.Request) {
	if r.OriginRequest == nil {
		r.OriginRequest = request
	}
	get := request.URL.Query()

	r.OriginValues = get
	r.GET = make(map[string]string)

	for k := range get {

		str := request.URL.Query().Get(k)
		tmp, err := url.QueryUnescape(str)
		if err != nil {
			log.Println(err.Error())
			r.GET[k] = str
			r.REQUEST[k] = str
		} else {
			r.GET[k] = tmp
			r.REQUEST[k] = tmp
		}
	}
}

func (r *Request) SyncPostData(request *http.Request, mem int64) {
	if r.OriginRequest == nil {
		r.OriginRequest = request
	}
	request.ParseForm()
	request.ParseMultipartForm(mem)
	r.POST = make(map[string]string)

	post := request.PostForm

	for k := range post {
		r.OriginValues[k] = post[k]
		str := request.PostFormValue(k)
		tmp, err := url.QueryUnescape(str)
		if err != nil {
			log.Println(err.Error())
			r.POST[k] = str
			r.REQUEST[k] = str
		} else {
			r.POST[k] = tmp
			r.REQUEST[k] = tmp
		}
	}

	if request.MultipartForm != nil {
		r.FILES = request.MultipartForm.File
		if len(r.FILES) > 0 {
			for fe := range r.FILES {
				for fk := range r.FILES[fe] {
					r.FILE = r.FILES[fe][fk]
				}
			}
		}

		mf := request.MultipartForm.Value
		for k := range mf {
			r.OriginValues[k] = mf[k]
			if len(mf[k]) > 0 {
				r.POST[k] = mf[k][0]
				r.REQUEST[k] = mf[k][0]
			}
		}
	}

	body, _ := ioutil.ReadAll(request.Body)
	r.BODY = string(body)

	rr := map[string]string{}
	json.Unmarshal([]byte(r.BODY), &rr)

	for e := range rr {
		r.POST[e] = rr[e]
		r.REQUEST[e] = rr[e]
	}
}

func (r *Request) SyncHeaderData(request *http.Request) {
	if r.OriginRequest == nil {
		r.OriginRequest = request
	}
	r.HEADER = make(map[string]string)
	header := request.Header
	for k := range header {
		if len(header[k]) > 0 {
			r.HEADER[k] = header[k][0]
		}
	}

}

func (r *Request) SyncCookieData(request *http.Request) {
	if r.OriginRequest == nil {
		r.OriginRequest = request
	}
	cookie := request.Cookies()
	r.COOKIE = make(map[string]string)
	for k := range cookie {
		r.COOKIE[cookie[k].Name] = cookie[k].Value
	}
}

func UpgradeRequest(request *http.Request) Request {
	r := &Request{
		GET:           make(map[string]string),
		POST:          make(map[string]string),
		REQUEST:       make(map[string]string),
		COOKIE:        make(map[string]string),
		SESSION:       make(map[string]string),
		HEADER:        make(map[string]string),
		FILES:         make(map[string][]*multipart.FileHeader),
		BODY:          "{}",
		FILE:          nil,
		OriginRequest: request,
		OriginValues:  url.Values{},
	}
	r.SyncGetData(request)
	r.SyncPostData(request, 0)
	r.SyncHeaderData(request)
	r.SyncCookieData(request)
	return *r
}

type responseData struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

type Response struct {
	OriginResponseWriter http.ResponseWriter
	displayCallback      func(data []byte, code int)
	code                 int
}

func (r *Response) displayByRaw(data []byte) {

	r.OriginResponseWriter.Header().Set("Access-Control-Allow-Credentials", "true")
	r.OriginResponseWriter.Header().Set("Access-Control-Allow-Methods", "Access-Control-Allow-Methods")
	r.OriginResponseWriter.Header().Set("Access-Control-Allow-Headers", "Origin, No-Cache, X-Requested-With, If-Modified-Since, Pragma, Last-Modified, Cache-Control, Expires, Content-Type, X-E4M-With")

	_, err := r.OriginResponseWriter.Write(data)
	if err != nil {
		log.Println("DisplayByRaw error:", string(data))
	}
	if r.displayCallback != nil {
		r.displayCallback(data, r.code)
	}

}

func (r *Response) DisplayByRaw(data []byte) {
	r.displayByRaw(data)
	panic("ok")
}

func (r *Response) DisplayByRawCache(data []byte, code int) {

	r.OriginResponseWriter.Header().Set("Access-Control-Allow-Credentials", "true")
	r.OriginResponseWriter.Header().Set("Access-Control-Allow-Methods", "Access-Control-Allow-Methods")
	r.OriginResponseWriter.Header().Set("Access-Control-Allow-Headers", "Origin, No-Cache, X-Requested-With, If-Modified-Since, Pragma, Last-Modified, Cache-Control, Expires, Content-Type, X-E4M-With")

	if code > 0 {
		r.OriginResponseWriter.WriteHeader(code)
	}
	_, _ = r.OriginResponseWriter.Write(data)

}

func (r *Response) DisplayCallback(displayCallback func(data []byte, code int)) {
	r.displayCallback = displayCallback
}

func (r *Response) DisplayByString(data string) {
	r.DisplayByRaw([]byte(data))
}

func (r *Response) Display(data interface{}, msg string, code int) {
	result := responseData{code, data, msg}
	text, err := json.Marshal(result)
	if err != nil {
		r.OriginResponseWriter.WriteHeader(500)
		r.code = 500
		r.DisplayByString("服务器异常:" + err.Error())
	}
	r.DisplayByRaw(text)
}

func (r *Response) displayByError(msg string, code int, data ...interface{}) {
	result := responseData{code, data, msg}
	text, err := json.Marshal(result)
	if err != nil {
		r.Display(nil, "JSON返回格式解析异常:"+err.Error(), 500)
	}
	r.displayByRaw(text)
}

func (r *Response) DisplayByError(msg string, code int, data ...interface{}) {
	result := responseData{code, data, msg}
	text, err := json.Marshal(result)
	if err != nil {
		r.Display(nil, "JSON返回格式解析异常:"+err.Error(), 500)
	}
	r.DisplayByRaw(text)
}

func (r *Response) CheckErrDisplayByError(err error, msg ...string) {
	if err == nil {
		return
	}
	if len(msg) > 0 {
		r.DisplayByError(strings.Join(msg, ","), 504)
	} else {
		r.DisplayByError(err.Error(), 504)
	}
}

func (r *Response) DisplayBySuccess(msgs ...string) {
	msg := "success"
	if len(msgs) > 0 {
		msg = msgs[0]
	}
	result := responseData{0, nil, msg}
	text, err := json.Marshal(result)
	if err != nil {
		r.Display(nil, "JSON返回格式解析异常:"+err.Error(), 500)
	}
	r.DisplayByRaw(text)
}

func (r *Response) DisplayByData(data interface{}) {
	result := responseData{0, data, ""}
	text, err := json.Marshal(result)
	if err != nil {
		r.Display(nil, "JSON返回格式解析异常:"+err.Error(), 500)
	}
	r.DisplayByRaw(text)
}

func (r *Response) SetCookie(name string, value string) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Secure:   false,
		HttpOnly: false,
	}
	http.SetCookie(r.OriginResponseWriter, cookie)
}

func (r *Response) SetHeader(name string, value string) {
	r.OriginResponseWriter.Header().Set(name, value)
}

func (r *Response) DisplayJPEG(img image.Image, o ...*jpeg.Options) {

	r.OriginResponseWriter.Header().Set("Access-Control-Allow-Credentials", "true")
	r.OriginResponseWriter.Header().Set("Access-Control-Allow-Methods", "Access-Control-Allow-Methods")
	r.OriginResponseWriter.Header().Set("Access-Control-Allow-Headers", "Origin, No-Cache, X-Requested-With, If-Modified-Since, Pragma, Last-Modified, Cache-Control, Expires, Content-Type, X-E4M-With")

	r.OriginResponseWriter.Header().Set("Content-Type", "image/jpeg")
	opt := &jpeg.Options{
		Quality: 95,
	}
	if len(o) > 0 {
		opt = o[0]
	}

	jpeg.Encode(r.OriginResponseWriter, img, opt)

	panic("ok")
}

func (r *Response) DisplayPNG(img image.Image) {

	r.OriginResponseWriter.Header().Set("Access-Control-Allow-Credentials", "true")
	r.OriginResponseWriter.Header().Set("Access-Control-Allow-Methods", "Access-Control-Allow-Methods")
	r.OriginResponseWriter.Header().Set("Access-Control-Allow-Headers", "Origin, No-Cache, X-Requested-With, If-Modified-Since, Pragma, Last-Modified, Cache-Control, Expires, Content-Type, X-E4M-With")

	r.OriginResponseWriter.Header().Set("Content-Type", "image/png")

	png.Encode(r.OriginResponseWriter, img)

	panic("ok")
}

func UpgradeResponse(w http.ResponseWriter) Response {
	return Response{OriginResponseWriter: w}
}

type HttpContext struct {
	Request
	Response
}

func (r *HttpContext) Body() gjson.Result {
	initData := r.BODY

	result := gjson.Parse(initData)

	for k, v := range r.REQUEST {
		if result.Get(k).Exists() {
			continue
		}

		initData, _ = sjson.Set(initData, k, v)
	}

	return gjson.Parse(initData)
}

func (r *HttpContext) ParamRequired(key string) string {
	if r.REQUEST[key] == "" {
		r.DisplayByError(fmt.Sprintf("参数错误,[%s]不允许为空", key), 404)
	}
	return r.REQUEST[key]
}

func (r *HttpContext) ParamRequired2Int(key string) int {
	s := r.ParamRequired(key)
	i, err := strconv.ParseInt(s, 10, 32)
	r.CheckErrDisplayByError(err)
	return int(i)
}

func (r *HttpContext) ParamRequired2Int64(key string) int64 {
	s := r.ParamRequired(key)
	i, err := strconv.ParseInt(s, 10, 64)
	r.CheckErrDisplayByError(err)
	return i
}

func (r *HttpContext) ParamRequired2Float(key string) float64 {
	s := r.ParamRequired(key)
	i, err := strconv.ParseFloat(s, 64)
	r.CheckErrDisplayByError(err)
	return i
}

func UpgradeHttpContext(w http.ResponseWriter, r *http.Request) *HttpContext {
	result := HttpContext{}
	result.Request = UpgradeRequest(r)
	result.Response = UpgradeResponse(w)
	return &result
}

var routeMap = map[string]func(ctx *HttpContext){}

func HandleAny(url string, callback func(ctx *HttpContext)) {
	routeMap[url] = callback
}

func HandleDo(url string, ctx *HttpContext) {
	defer func() {
		err := recover()
		if err == "ok" || err == nil || fmt.Sprint(err) == "" || fmt.Sprint(err) == "<nil>" {
			return
		}
		ctx.displayByError(fmt.Sprint(err), 500, debug.Stack())
	}()
	if routeMap[url] == nil {
		ctx.displayByError("接口不存在", 404)
		return
	}
	routeMap[url](ctx)
}
