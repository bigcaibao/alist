package controllers

import (
	"fmt"
	"github.com/Xhofe/alist/conf"
	"github.com/Xhofe/alist/drivers/base"
	"github.com/Xhofe/alist/model"
	"github.com/Xhofe/alist/server/common"
	"github.com/Xhofe/alist/utils"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"strings"
)

func Hide(files []model.File, path string) []model.File {
	meta, _ := model.GetMetaByPath(path)
	if meta != nil && meta.Hide != "" {
		tmpFiles := make([]model.File, 0)
		hideFiles := strings.Split(meta.Hide, ",")
		for _, item := range files {
			if !utils.IsContain(hideFiles, item.Name) {
				tmpFiles = append(tmpFiles, item)
			}
		}
		files = tmpFiles
	}
	return files
}

func Path(c *gin.Context) {
	reqV, _ := c.Get("req")
	req := reqV.(common.PathReq)
	if model.AccountsCount() > 1 && req.Path == "/" {
		files, err := model.GetAccountFiles()
		if err != nil {
			common.ErrorResp(c, err, 500)
			return
		}
		files = Hide(files, req.Path)
		c.JSON(200, common.Resp{
			Code:    200,
			Message: "folder",
			Data:    files,
		})
		return
	}
	account, path, driver, err := common.ParsePath(req.Path)
	if err != nil {
		common.ErrorResp(c, err, 500)
		return
	}
	file, files, err := driver.Path(path, account)
	if err != nil {
		common.ErrorResp(c, err, 500)
		return
	}
	if file != nil {
		// 对于中转文件或只能中转,将链接修改为中转链接
		if driver.Config().OnlyProxy || account.Proxy {
			if account.DownProxyUrl != "" {
				file.Url = fmt.Sprintf("%s%s?sign=%s", account.DownProxyUrl, req.Path, utils.SignWithToken(file.Name, conf.Token))
			} else {
				file.Url = fmt.Sprintf("//%s/p%s?sign=%s", c.Request.Host, req.Path, utils.SignWithToken(file.Name, conf.Token))
			}
		} else if !driver.Config().NoNeedSetLink {
			link, err := driver.Link(base.Args{Path: path, IP: c.ClientIP()}, account)
			if err != nil {
				common.ErrorResp(c, err, 500)
				return
			}
			file.Url = link.Url
		}
		c.JSON(200, common.Resp{
			Code:    200,
			Message: "file",
			Data:    []*model.File{file},
		})
	} else {
		files = Hide(files, req.Path)
		if driver.Config().LocalSort {
			model.SortFiles(files, account)
		}
		c.JSON(200, common.Resp{
			Code:    200,
			Message: "folder",
			Data:    files,
		})
	}
}

// Link 返回真实的链接，且携带头,只提供给中转程序使用
func Link(c *gin.Context) {
	var req common.PathReq
	if err := c.ShouldBind(&req); err != nil {
		common.ErrorResp(c, err, 400)
		return
	}
	req.Path = utils.ParsePath(req.Path)
	rawPath := req.Path
	rawPath = utils.ParsePath(rawPath)
	log.Debugf("link: %s", rawPath)
	account, path, driver, err := common.ParsePath(rawPath)
	if err != nil {
		common.ErrorResp(c, err, 500)
		return
	}
	if driver.Config().NoLink {
		common.SuccessResp(c, base.Link{
			Url: fmt.Sprintf("//%s/p%s?d=1&sign=%s", c.Request.Host, req.Path, utils.SignWithToken(utils.Base(rawPath), conf.Token)),
		})
		return
	}
	link, err := driver.Link(base.Args{Path: path, IP: c.ClientIP()}, account)
	if err != nil {
		common.ErrorResp(c, err, 500)
		return
	}
	common.SuccessResp(c, link)
	return
}

func Preview(c *gin.Context) {
	reqV, _ := c.Get("req")
	req := reqV.(common.PathReq)
	rawPath := req.Path
	rawPath = utils.ParsePath(rawPath)
	log.Debugf("preview: %s", rawPath)
	account, path, driver, err := common.ParsePath(rawPath)
	if err != nil {
		common.ErrorResp(c, err, 500)
		return
	}
	data, err := driver.Preview(path, account)
	if err != nil {
		common.ErrorResp(c, err, 500)
	} else {
		common.SuccessResp(c, data)
	}
}