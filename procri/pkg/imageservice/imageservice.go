package imageservice

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/peterbourgon/diskv"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/net/context"

	cri "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog"
)

type ImageService struct {
	// UUID generated when the service is started, to fake a storage UUID.
	uuid string
	// Persistent store for image and runtime data.
	dataStore *diskv.Diskv
}

type Image struct {
	Image         string   `json:"image"`
	Username      string   `json:"username"`
	Password      string   `json:"password"`
	Auth          string   `json:"auth"`
	ServerAddress string   `json:"serverAddress"`
	IdentityToken string   `json:"identityToken"`
	RegistryToken string   `json:"registryToken"`
	Tags          []string `json:",omitempty"`
	Digests       []string `json:",omitempty"`
}

func NewImageService(dataStore *diskv.Diskv) *ImageService {
	is := ImageService{
		uuid:      uuid.NewV4().String(),
		dataStore: dataStore,
	}
	return &is
}

func (is *ImageService) getImage(key string) *Image {
	buf, err := is.dataStore.Read(key)
	if err != nil {
		klog.V(5).Infof("looking up image %s: %v", key, err)
		return nil
	}

	img := Image{}
	err = json.Unmarshal(buf, &img)
	if err != nil {
		klog.Errorf("deserializing image data for %s: %v", key, err)
		return nil
	}
	if len(img.Tags) == 1 {
		if img.Tags[0] == "" {
			img.Tags = []string{}
		}

	}
	if len(img.Digests) == 1 {
		if img.Digests[0] == "" {
			img.Digests = []string{}
		}

	}

	return &img
}

func (is *ImageService) marshalAndSave(key string, img *Image) error {
	buf, err := json.Marshal(img)
	if err != nil {
		klog.Errorf("serializing image data for %s: %v", key, err)
		return err
	}

	err = is.dataStore.Write(key, buf)
	if err != nil {
		klog.V(5).Infof("storing image %s: %v", key, err)
		return err
	}
	return nil
}

func (is *ImageService) putImage(key string, img *Image) error {
	err := is.marshalAndSave(key, img)
	if err != nil {
		return fmt.Errorf("saving image %s: %v", key, err)
	}
	return nil
}

func (is *ImageService) deleteImage(key string) bool {
	err := is.dataStore.Erase(key)
	if err != nil {
		klog.Errorf("deleting image %s: %v", key, err)
		return false
	}

	return true
}

func (is *ImageService) listImages() []*Image {
	images := make([]*Image, 0)

	for key := range is.dataStore.Keys(nil) {
		klog.V(4).Infof("trying to get image under %s key", key)
		img := is.getImage(key)
		if img != nil {
			images = append(images, img)
		}
	}

	return images
}

//
// Implementation of cri.ImageService.
//

// ListImages lists existing images.
func (is *ImageService) ListImages(ctx context.Context, req *cri.ListImagesRequest) (*cri.ListImagesResponse, error) {
	klog.V(4).Infof("ListImages request %+v", req)

	resp := &cri.ListImagesResponse{
		Images: make([]*cri.Image, 0),
	}

	for _, img := range is.listImages() {
		klog.V(4).Infof("ListImages img: %v", img)
		if req.Filter == nil || req.Filter.Image == nil || req.Filter.Image.Image == img.Image {
			image := &cri.Image{
				Id:          img.Image,
				RepoTags:    img.Tags,
				RepoDigests: img.Digests,
				Size_:       0,
				Uid: &cri.Int64Value{
					Value: 0,
				},
				Username: "",
			}
			resp.Images = append(resp.Images, image)
		}
	}

	klog.V(4).Infof("ListImages request %+v: %+v", req, resp)
	return resp, nil
}

// ImageStatus returns the status of the image. If the image is not
// present, returns a response with ImageStatusResponse.Image set to
// nil.
func (is *ImageService) ImageStatus(ctx context.Context, req *cri.ImageStatusRequest) (*cri.ImageStatusResponse, error) {
	klog.V(4).Infof("ImageStatus request %+v", req)

	resp := cri.ImageStatusResponse{
		Image: nil,
	}

	if req.Image == nil {
		klog.V(5).Infof("ImageStatus request %v: %v", req, resp)
		return &resp, nil
	}
	imgName, _, _ := getImageNameTagAndDigest(req.Image.Image)
	klog.V(4).Infof("trying to getImage under %s key", imgName)

	if img := is.getImage(imgName); img != nil {
		klog.V(4).Infof("got image: %v", img)
		image := &cri.Image{
			Id:    req.Image.Image,
			Size_: 1, // This can't be zero.
			Uid: &cri.Int64Value{
				Value: 0,
			},
			Username: "",
		}
		if len(img.Tags) > 0 {
			klog.V(4).Infof("adding tags: %s", img.Tags)
			klog.V(4).Infof("adding tags with len: %v", len(img.Tags))
			klog.V(4).Infof("adding tag: %s", img.Tags[0])
			image.RepoTags = img.Tags
		}
		if len(img.Digests) > 0 {
			klog.V(4).Infof("adding digests: %s", img.Tags)
			image.RepoDigests = img.Digests
		}

		resp.Image = image
		return &resp, nil
	}

	klog.V(4).Infof("ImageStatus request %v: %v", req, resp)
	return &resp, nil
}

// PullImage pulls an image with authentication config.
func (is *ImageService) PullImage(ctx context.Context, req *cri.PullImageRequest) (*cri.PullImageResponse, error) {
	klog.V(4).Infof("PullImage request %+v", req)

	if req.Image == nil {
		err := fmt.Errorf("invalid PullImageRequest, Image is nil")
		klog.Errorf("PullImage request %v: %v", req, err)
		return nil, err
	}

	imageName, imageTag, imageDigest := getImageNameTagAndDigest(req.Image.Image)
	// check if image already exists
	img := is.getImage(imageName)
	var err error
	if img != nil {
		klog.V(4).Infof("add imageTag to img.Tags")
		tags := addToSliceWithoutDuplicate(imageTag, img.Tags)
		klog.V(4).Infof("add imageDigest to img.Digests")
		digests := addToSliceWithoutDuplicate(imageDigest, img.Digests)
		img.Tags = tags
		img.Digests = digests
		err = is.putImage(imageName, img)

	} else {
		klog.V(4).Infof("add imageTag to img.Tags")
		tags := addToSliceWithoutDuplicate(imageTag, []string{})
		klog.V(4).Infof("add imageDigest to img.Digests")
		digests := addToSliceWithoutDuplicate(imageDigest, []string{})
		image := Image{
			Image:   req.Image.Image,
			Tags:    tags,
			Digests: digests,
		}
		if req.Auth != nil {
			klog.V(4).Infof("PullImage authentication is needed for image %s", imageName)
			image.Auth = req.Auth.Auth
			image.Username = req.Auth.Username
			image.Password = req.Auth.Password
			image.ServerAddress = req.Auth.ServerAddress
			image.IdentityToken = req.Auth.IdentityToken
			image.RegistryToken = req.Auth.RegistryToken
		}

		err = is.putImage(imageName, &image)
	}
	if err != nil {
		return nil, err
	}
	resp := cri.PullImageResponse{
		ImageRef: imageName,
	}
	klog.V(4).Infof("PullImage request %v: %v", req, resp)
	return &resp, nil
}

// RemoveImage removes the image.
// This call is idempotent, and must not return an error if the image has
// already been removed.
func (is *ImageService) RemoveImage(ctx context.Context, req *cri.RemoveImageRequest) (*cri.RemoveImageResponse, error) {
	klog.V(4).Infof("RemoveImage request %+v", req)

	if req.Image == nil {
		err := fmt.Errorf("invalid RemoveImageRequest, Image is nil")
		klog.Errorf("RemoveImage request %v: %v", req, err)
		return nil, err
	}
	imageName, imageTag, imageDigest := getImageNameTagAndDigest(req.Image.Image)
	klog.V(4).Infof("RemoveImage: got %s to remove, key: %s tag: %s digest: %s", req.Image.Image, imageName, imageTag, imageDigest)
	img := is.getImage(imageName)
	if img != nil {
		// case: image exists and has only one tag
		if (len(img.Tags) == 1 && img.Tags[0] == imageTag) || len(img.Digests) == 1 && img.Digests[0] == imageDigest {
			is.deleteImage(imageName)
			resp := cri.RemoveImageResponse{}
			klog.V(4).Infof("RemoveImage request %v: %v", req, resp)
			return &resp, nil
		}
		// case: there are multiple tags for this image. Go over the tag list,
		// remove one specified in request and update image entry
		newTagList := removeFromSlice(imageTag, img.Tags)
		newDigestsList := removeFromSlice(imageDigest, img.Digests)
		img.Tags = newTagList
		img.Digests = newDigestsList
		klog.V(4).Infof("tags or digests list changed, updating with tags: %s, digests: %s", img.Tags, img.Digests)
		err := is.putImage(imageName, img)
		if err != nil {
			return nil, err
		}
		return &cri.RemoveImageResponse{}, nil

	}
	klog.Warningf("RemoveImage request %v: unknown image %s", req, imageName)
	return &cri.RemoveImageResponse{}, nil
}

// ImageFSInfo returns information of the filesystem that is used to store
// images.
func (is *ImageService) ImageFsInfo(ctx context.Context, req *cri.ImageFsInfoRequest) (*cri.ImageFsInfoResponse, error) {
	klog.V(4).Infof("ImageFsInfo request %+v", req)

	fu := cri.FilesystemUsage{
		Timestamp: time.Now().UnixNano(),
		FsId: &cri.FilesystemIdentifier{
			Mountpoint: "/",
		},
		UsedBytes: &cri.UInt64Value{
			Value: 0,
		},
		InodesUsed: &cri.UInt64Value{
			Value: 0,
		},
	}

	resp := cri.ImageFsInfoResponse{
		ImageFilesystems: []*cri.FilesystemUsage{
			&fu,
		},
	}

	klog.V(4).Infof("ImageFsInfo request %v: %v", req, resp)
	return &resp, nil
}

func getImageNameTagAndDigest(image string) (string, string, string) {
	defaultTag := ":latest"
	if strings.Contains(image, "@sha256") {
		imageParts := strings.Split(image, "@")
		return makeImgKey(imageParts[0]), "", image
	}
	imageParts := strings.Split(image, ":")
	if len(imageParts) > 1 {
		imageName := imageParts[0]
		return makeImgKey(imageName), image, ""
	}
	return makeImgKey(imageParts[0]), imageParts[0] + defaultTag, ""
}

func makeImgKey(imgName string) string {
	// This is a workaround for issue with saving images in format
	// <host>/<path>/<img-name> which results in attempt to create
	// nested folders and directories.
	hash := sha256.Sum256([]byte(imgName))
	return hex.EncodeToString(hash[:])
}

func addToSliceWithoutDuplicate(item string, slice []string) []string {
	if item == "" {
		return slice
	}
	for _, i := range slice {
		if i == item {
			klog.V(4).Infof("item %s == %s", i, item)
			return slice
		}
	}
	slice = append(slice, item)
	klog.V(4).Infof("added item: %s to slice: %v", item, slice)
	return slice
}

func removeFromSlice(item string, slice []string) []string {
	var updatedSlice []string
	klog.V(4).Infof("trying to remove %s from %s", item, slice)
	for _, i := range slice {
		if i != item {
			updatedSlice = append(updatedSlice, i)
		}
	}
	klog.V(4).Infof("updated slice: %s", updatedSlice)
	return updatedSlice
}
