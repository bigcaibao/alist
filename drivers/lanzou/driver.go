package lanzou

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/go-resty/resty/v2"
)

var upClient = base.NewRestyClient().SetTimeout(120 * time.Second)

type LanZou struct {
	Addition
	model.Storage
	uid string
}

func (d *LanZou) Config() driver.Config {
	return config
}

func (d *LanZou) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *LanZou) Init(ctx context.Context) error {
	if d.IsCookie() {
		if d.RootFolderID == "" {
			d.RootFolderID = "-1"
		}
		ylogin := regexp.MustCompile("ylogin=(.*?);").FindStringSubmatch(d.Cookie)
		if len(ylogin) < 2 {
			return fmt.Errorf("cookie does not contain ylogin")
		}
		d.uid = ylogin[1]
	}
	return nil
}

func (d *LanZou) Drop(ctx context.Context) error {
	d.uid = ""
	return nil
}

// 获取的大小和时间不准确
func (d *LanZou) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	if d.IsCookie() {
		return d.GetAllFiles(dir.GetID())
	} else {
		return d.GetFileOrFolderByShareUrl(dir.GetID(), d.SharePassword)
	}
}

func (d *LanZou) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	var (
		err   error
		dfile *FileOrFolderByShareUrl
	)
	switch file := file.(type) {
	case *FileOrFolder:
		// 先获取分享链接
		sfile := file.GetShareInfo()
		if sfile == nil {
			sfile, err = d.getFileShareUrlByID(file.GetID())
			if err != nil {
				return nil, err
			}
			file.SetShareInfo(sfile)
		}

		// 然后获取下载链接
		dfile, err = d.GetFilesByShareUrl(sfile.FID, sfile.Pwd)
		if err != nil {
			return nil, err
		}
		// 修复文件大小
		if d.RepairFileInfo && !file.repairFlag {
			size, time := d.getFileRealInfo(dfile.Url)
			if size != nil {
				file.size = size
				file.repairFlag = true
			}
			if file.time != nil {
				file.time = time
			}
		}
	case *FileOrFolderByShareUrl:
		dfile, err = d.GetFilesByShareUrl(file.GetID(), file.Pwd)
		if err != nil {
			return nil, err
		}
		// 修复文件大小
		if d.RepairFileInfo && !file.repairFlag {
			size, time := d.getFileRealInfo(dfile.Url)
			if size != nil {
				file.size = size
				file.repairFlag = true
			}
			if file.time != nil {
				file.time = time
			}
		}
	}
	exp := GetExpirationTime(dfile.Url)
	return &model.Link{
		URL: dfile.Url,
		Header: http.Header{
			"User-Agent": []string{base.UserAgent},
		},
		Expiration: &exp,
	}, nil
}

func (d *LanZou) MakeDir(ctx context.Context, parentDir model.Obj, dirName string) (model.Obj, error) {
	if d.IsCookie() {
		data, err := d.doupload(func(req *resty.Request) {
			req.SetContext(ctx)
			req.SetFormData(map[string]string{
				"task":               "2",
				"parent_id":          parentDir.GetID(),
				"folder_name":        dirName,
				"folder_description": "",
			})
		}, nil)
		if err != nil {
			return nil, err
		}
		return &FileOrFolder{
			Name:  dirName,
			FolID: utils.Json.Get(data, "text").ToString(),
		}, nil
	}
	return nil, errs.NotImplement
}

func (d *LanZou) Move(ctx context.Context, srcObj, dstDir model.Obj) (model.Obj, error) {
	if d.IsCookie() {
		if !srcObj.IsDir() {
			_, err := d.doupload(func(req *resty.Request) {
				req.SetContext(ctx)
				req.SetFormData(map[string]string{
					"task":      "20",
					"folder_id": dstDir.GetID(),
					"file_id":   srcObj.GetID(),
				})
			}, nil)
			if err != nil {
				return nil, err
			}
			return srcObj, nil
		}
	}
	return nil, errs.NotImplement
}

func (d *LanZou) Rename(ctx context.Context, srcObj model.Obj, newName string) (model.Obj, error) {
	if d.IsCookie() {
		if !srcObj.IsDir() {
			_, err := d.doupload(func(req *resty.Request) {
				req.SetContext(ctx)
				req.SetFormData(map[string]string{
					"task":      "46",
					"file_id":   srcObj.GetID(),
					"file_name": newName,
					"type":      "2",
				})
			}, nil)
			if err != nil {
				return nil, err
			}
			srcObj.(*FileOrFolder).NameAll = newName
			return srcObj, nil
		}
	}
	return nil, errs.NotImplement
}

func (d *LanZou) Remove(ctx context.Context, obj model.Obj) error {
	if d.IsCookie() {
		_, err := d.doupload(func(req *resty.Request) {
			req.SetContext(ctx)
			if obj.IsDir() {
				req.SetFormData(map[string]string{
					"task":      "3",
					"folder_id": obj.GetID(),
				})
			} else {
				req.SetFormData(map[string]string{
					"task":    "6",
					"file_id": obj.GetID(),
				})
			}
		}, nil)
		return err
	}
	return errs.NotImplement
}

func (d *LanZou) Put(ctx context.Context, dstDir model.Obj, stream model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	if d.IsCookie() {
		var resp RespText[[]FileOrFolder]
		_, err := d._post(d.BaseUrl+"/fileup.php", func(req *resty.Request) {
			req.SetFormData(map[string]string{
				"task":      "1",
				"vie":       "2",
				"ve":        "2",
				"id":        "WU_FILE_0",
				"name":      stream.GetName(),
				"folder_id": dstDir.GetID(),
			}).SetFileReader("upload_file", stream.GetName(), stream).SetContext(ctx)
		}, &resp, true)
		if err != nil {
			return nil, err
		}
		return &resp.Text[0], nil
	}
	return nil, errs.NotImplement
}
