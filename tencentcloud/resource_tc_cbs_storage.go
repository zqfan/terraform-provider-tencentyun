package tencentcloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/zqfan/tencentcloud-sdk-go/client"
)

const MaxStorageNameLength = 60

const (
	BasicStorageMinimumSize   = 10
	PremiumStorageMinimumSize = 50
	SsdStorageMinimumSize     = 100
	StorageMaxSize            = 4000
)

const (
	tencentCloudApiStorageTypeBasic   = "cloudBasic"
	tencentCloudApiStorageTypePremium = "cloudPremium"
	tencentCloudApiStorageTypeSSD     = "cloudSSD"
)

var (
	availableStorageTypeFamilies = []string{
		tencentCloudApiStorageTypeBasic,
		tencentCloudApiStorageTypePremium,
		tencentCloudApiStorageTypeSSD,
	}
)

var (
	availablePeriodValue = []int{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 24, 36, 48, 60,
	}
)

var (
	errStorageNotFound = errors.New("storage not found")
)

type storageInfo struct {
	StorageType   string `json:"storageType"`
	StorageSize   int    `json:"storageSize"`
	Zone          string `json:"zone"`
	StorageName   string `json:"storageName"`
	StorageStatus string `json:"storageStatus"`
	Attached      int    `json:"attached"`
	InstanceId    string `json:"uInstanceId"`
}

func resourceTencentCloudCbsStorage() *schema.Resource {
	return &schema.Resource{
		Create: resourceTencentCloudCbsStorageCreate,
		Read:   resourceTencentCloudCbsStorageRead,
		Update: resourceTencentCloudCbsStorageUpdate,
		Delete: resourceTencentCloudCbsStorageDelete,

		Schema: map[string]*schema.Schema{
			"storage_type": &schema.Schema{
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validateStorageType,
			},
			"storage_size": &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
			},
			"period": &schema.Schema{
				Type:         schema.TypeInt,
				Required:     true,
				ValidateFunc: validateStoragePeriod,
			},
			"availability_zone": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"storage_name": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateStorageName,
			},
			"storage_status": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"attached": &schema.Schema{
				Type:     schema.TypeInt,
				Computed: true,
			},
			"snapshot_id": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func modifyCbsStorage(storageId string, storageName string, m interface{}) error {
	client := m.(*TencentCloudClient).commonConn
	params := map[string]string{
		"Action":      "ModifyCbsStorageAttributes",
		"storageId":   storageId,
		"storageName": storageName,
	}

	response, err := client.SendRequest("cbs", params)
	if err != nil {
		return err
	}
	var jsonresp struct {
		Code     int    `json:"code"`
		Message  string `json:"message"`
		CodeDesc string `json:"codeDesc"`
	}
	err = json.Unmarshal([]byte(response), &jsonresp)
	if err != nil {
		return err
	}
	if jsonresp.Code != 0 {
		return fmt.Errorf(
			"ModifyCbsStorageAttributes error, code:%v, message: %v, codeDesc: %v.",
			jsonresp.Code,
			jsonresp.Message,
			jsonresp.CodeDesc,
		)
	}

	log.Printf("[DEBUG] ModifyCbsStorageAttributes, new storageName: %#v.", storageName)
	return nil
}

func rollbackStorage(storageId string, snapshotId string, m interface{}) error {
	client := m.(*TencentCloudClient).commonConn
	params := map[string]string{
		"Action":     "ApplySnapshot",
		"storageId":  storageId,
		"snapshotId": snapshotId,
	}

	response, err := client.SendRequest("snapshot", params)
	if err != nil {
		return err
	}
	var jsonresp struct {
		Code     int
		Message  string
		CodeDesc string
	}
	err = json.Unmarshal([]byte(response), &jsonresp)
	if err != nil {
		return err
	}
	if jsonresp.Code != 0 {
		return fmt.Errorf(
			"rollback storage error, code:%v, message: %v, codeDesc: %v.",
			jsonresp.Code,
			jsonresp.Message,
			jsonresp.CodeDesc,
		)
	}

	log.Printf("[DEBUG] rollback storage %#v with snapshot %#v.", storageId, snapshotId)
	return nil
}

func terminateCbsStorage(storageId string, client *client.Client) *resource.RetryError {
	params := map[string]string{
		"Action":       "TerminateCbsStorages",
		"storageIds.0": storageId,
	}

	response, err := client.SendRequest("cbs", params)
	if err != nil {
		return resource.NonRetryableError(err)
	}
	var jsonresp struct {
		Code     interface{}
		Message  string
		CodeDesc string
	}
	err = json.Unmarshal([]byte(response), &jsonresp)
	if err != nil {
		return resource.NonRetryableError(err)
	}
	code, ok := jsonresp.Code.(float64)
	// the code maybe a string
	if !ok || code != 0 {
		if strings.Contains(jsonresp.Message, "query deal & resourceDeal fail") {
			// for a new disk, we can terminate after a few minutes.
			return resource.RetryableError(fmt.Errorf("query deal failed, please retry later"))
		}
		return resource.NonRetryableError(fmt.Errorf(
			"terminate storage error, message: %v, codeDesc: %v.",
			jsonresp.Message,
			jsonresp.CodeDesc,
		))
	}
	return nil
}

func describeCbsStorage(storageId string, client *client.Client) (*storageInfo, bool, error) {
	var jsonresp struct {
		Code       int    `json:"code"`
		Message    string `json:"message"`
		CodeDesc   string `json:"codeDesc"`
		StorageSet []storageInfo
	}
	params := map[string]string{
		"Action":       "DescribeCbsStorages",
		"storageIds.0": storageId,
	}
	response, err := client.SendRequest("cbs", params)
	canRetryError := false
	if err != nil {
		return nil, canRetryError, err
	}
	err = json.Unmarshal([]byte(response), &jsonresp)
	if err != nil {
		return nil, canRetryError, err
	}
	if jsonresp.Code != 0 {
		return nil, canRetryError, fmt.Errorf(
			"DescribeCbsStorages error, code:%v, message: %v, codeDesc: %v.",
			jsonresp.Code,
			jsonresp.Message,
			jsonresp.CodeDesc,
		)
	}

	if len(jsonresp.StorageSet) == 0 {
		canRetryError = true
		return nil, canRetryError, errStorageNotFound

	}

	storage := jsonresp.StorageSet[0]
	return &storage, canRetryError, nil
}

func resourceTencentCloudCbsStorageCreate(d *schema.ResourceData, m interface{}) error {
	client := m.(*TencentCloudClient).commonConn
	params := map[string]string{
		"Action":      "CreateCbsStorages",
		"storageType": d.Get("storage_type").(string),
		"period":      strconv.Itoa(d.Get("period").(int)),
		"zone":        d.Get("availability_zone").(string),
		"payMode":     "prePay",
		"goodsNum":    "1",
	}

	size := d.Get("storage_size").(int)
	if size%10 != 0 {
		return fmt.Errorf("Storage_size: %v is illegal, must be an integer of 10", size)
	}
	storageType := d.Get("storage_type").(string)
	if storageType == tencentCloudApiStorageTypeBasic &&
		(size < BasicStorageMinimumSize || size > StorageMaxSize) {
		return fmt.Errorf(
			"The size of cloud basic storage must between %v to %v.",
			BasicStorageMinimumSize,
			StorageMaxSize,
		)
	}

	if storageType == tencentCloudApiStorageTypePremium &&
		(size < PremiumStorageMinimumSize || size > StorageMaxSize) {
		return fmt.Errorf(
			"The size of cloud basic storage must between %v to %v.",
			PremiumStorageMinimumSize,
			StorageMaxSize,
		)
	}

	if storageType == tencentCloudApiStorageTypeSSD &&
		(size < SsdStorageMinimumSize || size > StorageMaxSize) {
		return fmt.Errorf(
			"The size of cloud basic storage must between %v to %v.",
			SsdStorageMinimumSize,
			StorageMaxSize,
		)
	}
	params["storageSize"] = strconv.Itoa(size)

	snapshotIdInf, ok := d.GetOk("snapshot_id")
	if ok {
		params["snapshotId"] = snapshotIdInf.(string)
	}

	response, err := client.SendRequest("cbs", params)
	if err != nil {
		return err
	}
	var jsonresp struct {
		Code       int      `json:"code"`
		Message    string   `json:"message"`
		CodeDesc   string   `json:"codeDesc"`
		StorageIds []string `json:"storageIds"`
	}
	err = json.Unmarshal([]byte(response), &jsonresp)
	if err != nil {
		return err
	}
	if jsonresp.Code != 0 {
		return fmt.Errorf(
			"CreateCbsStorages error, code:%v, message:%v, codeDesc:%v.",
			jsonresp.Code,
			jsonresp.Message,
			jsonresp.CodeDesc,
		)
	}
	storageId := jsonresp.StorageIds[0]
	d.SetId(storageId)
	time.Sleep(time.Second * 3)
	resource.Retry(3*time.Minute, func() *resource.RetryError {
		_, canRetryError, err := describeCbsStorage(d.Id(), m.(*TencentCloudClient).commonConn)
		if err != nil {
			if canRetryError == false {
				return resource.NonRetryableError(err)
			} else {
				return resource.RetryableError(fmt.Errorf("Storage is creating..."))
			}
		}

		return nil
	})
	log.Printf("[DEBUG] CreateCbsStorages success - storageId: %#v.", storageId)
	//TODO 由于CreateCbsStorages接口不支持创建时设置云盘名称，所以在创建完后需设置云盘名称
	if storageName, ok := d.GetOk("storage_name"); ok {
		err = modifyCbsStorage(d.Id(), storageName.(string), m)
		if err != nil {
			return err
		}
	}
	return resourceTencentCloudCbsStorageRead(d, m)
}

func resourceTencentCloudCbsStorageRead(d *schema.ResourceData, m interface{}) error {
	storage, _, err := describeCbsStorage(d.Id(), m.(*TencentCloudClient).commonConn)
	if err != nil {
		if err == errStorageNotFound {
			d.SetId("")
			return nil
		}
		return err
	}
	d.Set("storage_type", storage.StorageType)
	d.Set("storage_size", storage.StorageSize)
	d.Set("availability_zone", storage.Zone)
	d.Set("storage_name", storage.StorageName)
	d.Set("storage_status", storage.StorageStatus)
	d.Set("attached", storage.StorageStatus)
	return nil
}

func resourceTencentCloudCbsStorageUpdate(d *schema.ResourceData, m interface{}) error {
	immutableItems := [...]string{"storage_size", "storage_type", "availability_zone", "period"}
	for _, item := range immutableItems {
		if d.HasChange(item) {
			return fmt.Errorf("[ERROR] %v does not support modification, please create a new disk instead.", item)
		}
	}

	requestUpdate := false
	if d.HasChange("storage_name") {
		_, n := d.GetChange("storage_name")
		storageName := n.(string)
		if storageName == "" {
			return fmt.Errorf("storage_name are not allow to be empty")
		}

		err := modifyCbsStorage(d.Id(), storageName, m)
		if err != nil {
			return err
		}
		requestUpdate = true
	}

	if d.HasChange("snapshot_id") {
		_, snapshotIdInf := d.GetChange("snapshot_id")
		snapshotId := snapshotIdInf.(string)
		if snapshotId == "" {
			// pass
		} else {
			err := rollbackStorage(d.Id(), snapshotId, m)
			if err != nil {
				return err
			}
		}
	}
	if requestUpdate {
		return resourceTencentCloudCbsStorageRead(d, m)
	}
	return nil
}

func resourceTencentCloudCbsStorageDelete(d *schema.ResourceData, m interface{}) error {
	err := resource.Retry(5*time.Minute, func() *resource.RetryError {
		return terminateCbsStorage(d.Id(), m.(*TencentCloudClient).commonConn)
	})
	d.SetId("")
	return err
}
