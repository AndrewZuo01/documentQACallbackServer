package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/OpenIMSDK/chat/pkg/common/apistruct"
	"github.com/OpenIMSDK/chat/pkg/proto/admin"
	"github.com/OpenIMSDK/chat/pkg/proto/chat"
	"github.com/OpenIMSDK/protocol/constant"
	"github.com/OpenIMSDK/protocol/msg"
	"github.com/OpenIMSDK/tools/apiresp"
	"github.com/OpenIMSDK/tools/errs"
	"github.com/OpenIMSDK/tools/log"
	"github.com/OpenIMSDK/tools/utils"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strings"
	"time"
)

func CallbackExample(c *gin.Context) {

	// 1. Callback after sending a single chat message
	var req CallbackAfterSendSingleMsgReq

	if err := c.BindJSON(&req); err != nil {
		log.ZError(c, "CallbackExample BindJSON failed", err)
		apiresp.GinError(c, errs.ErrArgs.WithDetail(err.Error()).Wrap())
		return
	}

	resp := CallbackAfterSendSingleMsgResp{
		CommonCallbackResp: CommonCallbackResp{
			ActionCode: 0,
			ErrCode:    200,
			ErrMsg:     "success",
			ErrDlt:     "successful",
			NextCode:   0,
		},
	}
	c.JSON(http.StatusOK, resp)
	fmt.Println("CallbackExample step1")
	// 2. If the user receiving the message is a customer service bot, return the message.

	// UserID of the robot account

	if req.SendID == "1930812794" || req.RecvID != "1930812794" {
		return
	}

	if req.ContentType != constant.Picture && req.ContentType != constant.Text {
		return
	}

	// Administrator token
	url := "http://127.0.0.1:10009/account/login"
	adminID := "admin1"
	password := md5.Sum([]byte(adminID))
	fmt.Println("CallbackExample step2")
	adminInput := admin.LoginReq{
		Account:  "admin1",
		Password: hex.EncodeToString(password[:]),
	}

	header := make(map[string]string, 2)
	header["operationID"] = "111"
	fmt.Println("CallbackExample step3")
	b, err := Post(c, url, header, adminInput, 10)
	if err != nil {
		log.ZError(c, "CallbackExample send message failed", err)
		apiresp.GinError(c, errs.ErrInternalServer.WithDetail(err.Error()).Wrap())
		return
	}

	type TokenInfo struct {
		ErrCode int                      `json:"errCode"`
		ErrMsg  string                   `json:"errMsg"`
		ErrDlt  string                   `json:"errDlt"`
		Data    apistruct.AdminLoginResp `json:"data,omitempty"`
	}
	fmt.Println("CallbackExample step4")
	adminOutput := &TokenInfo{}

	if err = json.Unmarshal(b, adminOutput); err != nil {
		log.ZError(c, "CallbackExample unmarshal failed", err)
		apiresp.GinError(c, errs.ErrInternalServer.WithDetail(err.Error()).Wrap())
		return
	}

	header["token"] = adminOutput.Data.AdminToken
	url = "http://127.0.0.1:10008/user/find/public"

	searchInput := chat.FindUserFullInfoReq{
		UserIDs: []string{"1930812794"},
	}
	fmt.Println("CallbackExample step5")
	b, err = Post(c, url, header, searchInput, 10)
	if err != nil {
		log.ZError(c, "CallbackExample unmarshal failed", err)
		apiresp.GinError(c, errs.ErrInternalServer.WithDetail(err.Error()).Wrap())
		return
	}

	type UserInfo struct {
		ErrCode int                       `json:"errCode"`
		ErrMsg  string                    `json:"errMsg"`
		ErrDlt  string                    `json:"errDlt"`
		Data    chat.FindUserFullInfoResp `json:"data,omitempty"`
	}

	searchOutput := &UserInfo{}
	fmt.Println("CallbackExample step6")
	if err = json.Unmarshal(b, searchOutput); err != nil {
		log.ZError(c, "search_output unmarshal failed", err)
		apiresp.GinError(c, errs.ErrInternalServer.WithDetail(err.Error()).Wrap())
		return
	}
	fmt.Println(searchOutput)
	if len(searchOutput.Data.Users) == 0 {
		fmt.Println("CallbackExample step6check")
		apiresp.GinError(c, errs.ErrRecordNotFound.Wrap("the robotics not found"))
		return
	}

	log.ZDebug(c, "callback", "searchUserAccount", searchOutput)
	fmt.Println("CallbackExample step7")
	text := TextElem{}
	picture := PictureElem{}
	mapStruct := make(map[string]any)
	// Processing text messages

	if err != nil {
		log.ZError(c, "CallbackExample get Sender failed", err)
		apiresp.GinError(c, errs.ErrInternalServer.WithDetail(err.Error()).Wrap())
		return
	}
	fmt.Println("CallbackExample step8")
	// Handle message structures
	if req.ContentType == constant.Text {
		err = json.Unmarshal([]byte(req.Content), &text)
		if err != nil {
			log.ZError(c, "CallbackExample unmarshal failed", err)
			apiresp.GinError(c, errs.ErrInternalServer.WithDetail(err.Error()).Wrap())
			return
		}
		log.ZDebug(c, "callback", "text", text)
		url = "http://127.0.0.1:5000/generate_QA_chain"
		questionData := map[string]string{"question": text.Content}
		b, err = Post(c, url, header, questionData, 1000)
		if err != nil {
			log.ZError(c, "CallbackExample unmarshal failed", err)
			apiresp.GinError(c, errs.ErrInternalServer.WithDetail(err.Error()).Wrap())
			return
		}
		var responseData AIResponse
		err = json.Unmarshal(b, &responseData)
		if err != nil {
			fmt.Println("error when unmarshal AI response")
		}

		mapStruct["content"] = responseData.Data
	} else {
		err = json.Unmarshal([]byte(req.Content), &picture)
		if err != nil {
			log.ZError(c, "CallbackExample unmarshal failed", err)
			apiresp.GinError(c, errs.ErrInternalServer.WithDetail(err.Error()).Wrap())
			return
		}
		log.ZDebug(c, "callback", "text", picture)
		if strings.Contains(picture.SourcePicture.Type, "/") {
			arr := strings.Split(picture.SourcePicture.Type, "/")
			picture.SourcePicture.Type = arr[1]
		}

		if strings.Contains(picture.BigPicture.Type, "/") {
			arr := strings.Split(picture.BigPicture.Type, "/")
			picture.BigPicture.Type = arr[1]
		}

		if len(picture.SnapshotPicture.Type) == 0 {
			picture.SnapshotPicture.Type = picture.SourcePicture.Type
		}

		mapStructSnap := make(map[string]interface{})
		if mapStructSnap, err = convertStructToMap(picture.SnapshotPicture); err != nil {
			log.ZError(c, "CallbackExample struct to map failed", err)
			apiresp.GinError(c, err)
			return
		}
		mapStruct["snapshotPicture"] = mapStructSnap

		mapStructBig := make(map[string]interface{})
		if mapStructBig, err = convertStructToMap(picture.BigPicture); err != nil {
			log.ZError(c, "CallbackExample struct to map failed", err)
			apiresp.GinError(c, err)
			return
		}
		mapStruct["bigPicture"] = mapStructBig

		mapStructSource := make(map[string]interface{})
		if mapStructSource, err = convertStructToMap(picture.SourcePicture); err != nil {
			log.ZError(c, "CallbackExample struct to map failed", err)
			apiresp.GinError(c, err)
			return
		}
		mapStruct["sourcePicture"] = mapStructSource
		mapStruct["sourcePath"] = picture.SourcePath
	}

	log.ZDebug(c, "callback", "mapStruct", mapStruct, "mapStructSnap")
	header["token"] = adminOutput.Data.ImToken
	fmt.Println("CallbackExample step9")
	input := &SendMsgReq{
		RecvID: req.SendID,
		SendMsg: SendMsg{
			SendID:           searchOutput.Data.Users[0].UserID,
			SenderNickname:   searchOutput.Data.Users[0].Nickname,
			SenderFaceURL:    searchOutput.Data.Users[0].FaceURL,
			SenderPlatformID: req.SenderPlatformID,
			Content:          mapStruct,
			ContentType:      req.ContentType,
			SessionType:      req.SessionType,
			SendTime:         utils.GetCurrentTimestampByMill(), // millisecond
		},
	}

	url = "http://127.0.0.1:10002/msg/send_msg"

	type sendResp struct {
		ErrCode int             `json:"errCode"`
		ErrMsg  string          `json:"errMsg"`
		ErrDlt  string          `json:"errDlt"`
		Data    msg.SendMsgResp `json:"data,omitempty"`
	}
	fmt.Println("CallbackExample step10")
	output := &sendResp{}

	// Initiate a post request that calls the interface that sends the message (the bot sends a message to user)
	b, err = Post(c, url, header, input, 10)
	if err != nil {
		log.ZError(c, "CallbackExample send message failed", err)
		apiresp.GinError(c, errs.ErrInternalServer.WithDetail(err.Error()).Wrap())
		return
	}
	if err = json.Unmarshal(b, output); err != nil {
		log.ZError(c, "CallbackExample unmarshal failed", err)
		apiresp.GinError(c, errs.ErrInternalServer.WithDetail(err.Error()).Wrap())
		return
	}
	res := &msg.SendMsgResp{
		ServerMsgID: output.Data.ServerMsgID,
		ClientMsgID: output.Data.ClientMsgID,
		SendTime:    output.Data.SendTime,
	}
	fmt.Println("CallbackExample step11")
	apiresp.GinSuccess(c, res)
}

func Post(ctx context.Context, url string, header map[string]string, data any, timeout int) (content []byte, err error) {
	var (
		// define http client.
		client = &http.Client{
			Timeout: 1000 * time.Second, // max timeout is 15s
		}
	)
	fmt.Println("post1")
	if timeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, time.Second*time.Duration(timeout))
		defer cancel()
	}
	fmt.Println("post2")
	jsonStr, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonStr))
	if err != nil {
		return nil, err
	}
	fmt.Println("post3")
	if operationID, _ := ctx.Value(constant.OperationID).(string); operationID != "" {
		req.Header.Set(constant.OperationID, operationID)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	req.Header.Add("content-type", "application/json; charset=utf-8")
	fmt.Println("post4")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer resp.Body.Close()
	fmt.Println("post5")
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func main() {
	// Create a Gin router
	r := gin.Default()

	// Define the route and handler
	r.POST("/callbackAfterSendSingleMsgCommand", CallbackExample) // Use POST method for your example

	// Start the server
	r.Run("0.0.0.0:8080") // Run on port 8080
}
