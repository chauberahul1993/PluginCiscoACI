//(C) Copyright [2020] Hewlett Packard Enterprise Development LP
//
//Licensed under the Apache License, Version 2.0 (the "License"); you may
//not use this file except in compliance with the License. You may obtain
//a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//License for the specific language governing permissions and limitations
// under the License.

//Package caphandler ...
package caphandler

import (
	"fmt"
	"github.com/ODIM-Project/ODIM/lib-dmtf/model"
	"github.com/ODIM-Project/ODIM/lib-utilities/common"
	"github.com/ODIM-Project/ODIM/lib-utilities/response"
	"github.com/ODIM-Project/PluginCiscoACI/capdata"
	"github.com/ODIM-Project/PluginCiscoACI/caputilities"
	aciModels "github.com/ciscoecosystem/aci-go-client/models"
	iris "github.com/kataras/iris/v12"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

// CreateApplicationProfile creates Application profiles using APIC
func CreateApplicationProfile(name string, tenant string, description string, fvApattr aciModels.ApplicationProfileAttributes) (*aciModels.ApplicationProfile, error) {
	aciServiceManager := caputilities.GetConnection()

	return aciServiceManager.CreateApplicationProfile(name, tenant, description, fvApattr)
}

// CreateVRF creates VRF's using APIC
func CreateVRF(name string, tenant string, description string, fvCtxattr aciModels.VRFAttributes) (*aciModels.VRF, error) {
	aciServiceManager := caputilities.GetConnection()
	return aciServiceManager.CreateVRF(name, tenant, description, fvCtxattr)
}

// GetZones returns the collection of zones present under a fabric
func GetZones(ctx iris.Context) {
	uri := ctx.Request().RequestURI
	fabricID := ctx.Params().Get("id")
	_, ok := capdata.FabricDataStore.Data[fabricID]
	if !ok {
		errMsg := fmt.Sprintf("Address data for uri %s not found", uri)
		log.Error(errMsg)
		resp := updateErrorResponse(response.ResourceNotFound, errMsg, []interface{}{"AddressPool", uri})
		ctx.StatusCode(http.StatusNotFound)
		ctx.JSON(resp)
		return
	}
	var members = []*model.Link{}

	for zoneID, zoneData := range capdata.ZoneDataStore {
		if zoneData.FabricID == fabricID {
			members = append(members, &model.Link{
				Oid: zoneID,
			})
		}
	}
	zoneCollection := model.Collection{
		ODataContext: "/ODIM/v1/$metadata#ZoneCollection.ZoneCollection",
		ODataID:      uri,
		ODataType:    "#ZoneCollection.ZoneCollection",
		Description:  "ZoneCollection view",
		Name:         "Zones",
		Members:      members,
		MembersCount: len(members),
	}
	ctx.StatusCode(http.StatusOK)
	ctx.JSON(zoneCollection)
}

// GetZone returns a specific zone present under a fabric
func GetZone(ctx iris.Context) {
	uri := ctx.Request().RequestURI
	fabricID := ctx.Params().Get("id")
	_, ok := capdata.FabricDataStore.Data[fabricID]
	if !ok {
		errMsg := fmt.Sprintf("Fabric data for uri %s not found", uri)
		log.Error(errMsg)
		resp := updateErrorResponse(response.ResourceNotFound, errMsg, []interface{}{"Fabric", fabricID})
		ctx.StatusCode(http.StatusNotFound)
		ctx.JSON(resp)
		return
	}

	respData, ok := capdata.ZoneDataStore[uri]
	if !ok {
		errMsg := fmt.Sprintf("Zone data for uri %s not found", uri)
		log.Error(errMsg)
		resp := updateErrorResponse(response.ResourceNotFound, errMsg, []interface{}{"Zone", fabricID})
		ctx.StatusCode(http.StatusNotFound)
		ctx.JSON(resp)
		return
	}
	ctx.StatusCode(http.StatusOK)
	ctx.JSON(respData.Zone)
}

// CreateZone default function called for creation of any type of zone
func CreateZone(ctx iris.Context) {
	uri := ctx.Request().RequestURI
	fabricID := ctx.Params().Get("id")
	_, ok := capdata.FabricDataStore.Data[fabricID]
	if !ok {
		errMsg := fmt.Sprintf("Fabric data for uri %s not found", uri)
		log.Error(errMsg)
		resp := updateErrorResponse(response.ResourceNotFound, errMsg, []interface{}{"Fabric", fabricID})
		ctx.StatusCode(http.StatusNotFound)
		ctx.JSON(resp)
		return
	}

	var zone model.Zone
	err := ctx.ReadJSON(&zone)
	if err != nil {
		errorMessage := "error while trying to get JSON body from the  request: " + err.Error()
		log.Error(errorMessage)
		resp := updateErrorResponse(response.MalformedJSON, errorMessage, nil)
		ctx.StatusCode(http.StatusBadRequest)
		ctx.JSON(resp)
		return
	}
	switch zone.ZoneType {
	case "Default":
		resp, statusCode := CreateDefaultZone(zone)
		if statusCode != http.StatusCreated {
			ctx.StatusCode(statusCode)
			ctx.JSON(resp)
			return
		}
		conflictFlag := false
		var defaultZoneID string
		for _, value := range capdata.ZoneDataStore {
			if value.Zone.Name == zone.Name {
				conflictFlag = true
			}
		}
		if !conflictFlag {
			defaultZoneID = uuid.NewV4().String()
			zone = saveZoneData(defaultZoneID, uri, fabricID, zone)
		}
		common.SetResponseHeader(ctx, map[string]string{
			"Location": zone.ODataID,
		})
		ctx.StatusCode(statusCode)
		ctx.JSON(zone)
		return
	case "ZoneOfZones":
		defaultZoneLink, resp, statusCode := CreateZoneOfZones(uri, fabricID, zone)
		if statusCode != http.StatusCreated {
			ctx.StatusCode(statusCode)
			ctx.JSON(resp)
			return
		}
		conflictFlag := false
		var defaultZoneID string
		for _, value := range capdata.ZoneDataStore {
			if value.Zone.Name == zone.Name {
				conflictFlag = true
			}
		}
		if !conflictFlag {
			defaultZoneID = uuid.NewV4().String()
			zone = saveZoneData(defaultZoneID, uri, fabricID, zone)
		}
		updateZoneData(defaultZoneLink, zone)
		common.SetResponseHeader(ctx, map[string]string{
			"Location": zone.ODataID,
		})
		ctx.StatusCode(statusCode)
		ctx.JSON(zone)
		return
	case "ZoneOfEndpoints":
		resp, statusCode := createZoneOfEndpoints(uri, fabricID, zone)
		if statusCode != http.StatusCreated {
			ctx.StatusCode(statusCode)
			ctx.JSON(resp)
			return
		}
		zoneID := uuid.NewV4().String()
		zone = saveZoneData(zoneID, uri, fabricID, zone)
		//updateZoneData()
		ctx.StatusCode(statusCode)
		ctx.JSON(zone)
		return
	default:
		ctx.StatusCode(http.StatusNotImplemented)
		return
	}
}

func saveZoneData(defaultZoneID string, uri string, fabricID string, zone model.Zone) model.Zone {
	zone.ID = defaultZoneID
	zone.ODataContext = "/ODIM/v1/$metadata#Zone.Zone"
	zone.ODataType = "#Zone.v1_4_0.Zone"
	zone.ODataID = fmt.Sprintf("%s/%s", uri, defaultZoneID)
	zone.Status = &model.Status{}
	zone.Status.State = "Enabled"
	zone.Status.Health = "OK"
	if zone.Links != nil {
		if zone.Links.ContainedByZones != nil {
			zone.Links.ContainedByZonesCount = len(zone.Links.ContainedByZones)
		}
	}
	capdata.ZoneDataStore[zone.ODataID] = &capdata.ZoneData{
		FabricID: fabricID,
		Zone:     &zone,
	}
	return zone
}

// CreateDefaultZone creates a zone of type 'Default'
func CreateDefaultZone(zone model.Zone) (interface{}, int) {
	var tenantAttributesStruct aciModels.TenantAttributes
	tenantAttributesStruct.Name = zone.Name
	aciClient := caputilities.GetConnection()
	//var tenantList []*aciModels.Tenant
	tenantList, err := aciClient.ListTenant()
	if err != nil {
		errMsg := "Error while creating default Zone: " + err.Error()
		resp := updateErrorResponse(response.GeneralError, errMsg, nil)
		return resp, http.StatusBadRequest
	}
	for _, tenant := range tenantList {
		if tenant.TenantAttributes.Name == zone.Name {
			errMsg := "Default zone already exists with name: " + zone.Name
			resp := updateErrorResponse(response.ResourceAlreadyExists, errMsg, []interface{}{"DefaultZone", tenant.TenantAttributes.Name, zone.Name})
			return resp, http.StatusConflict
		}

	}

	resp, err := aciClient.CreateTenant(zone.Name, zone.Description, tenantAttributesStruct)
	if err != nil {
		errMsg := "Error while creating default Zone: " + err.Error()
		resp := updateErrorResponse(response.GeneralError, errMsg, nil)
		return resp, http.StatusBadRequest
	}
	return resp, http.StatusCreated
}

// DeleteZone deletes the zone from the resource
func DeleteZone(ctx iris.Context) {
	uri := ctx.Request().RequestURI
	fabricID := ctx.Params().Get("id")
	_, ok := capdata.FabricDataStore.Data[fabricID]
	if !ok {
		errMsg := fmt.Sprintf("Fabric data for uri %s not found", uri)
		log.Error(errMsg)
		resp := updateErrorResponse(response.ResourceNotFound, errMsg, []interface{}{"Fabric", fabricID})
		ctx.StatusCode(http.StatusNotFound)
		ctx.JSON(resp)
		return
	}

	//TODO: Get list of zones which are pre-populated from onstart and compare the members for item not present in odim but present in ACI

	respData, ok := capdata.ZoneDataStore[uri]
	log.Println(capdata.ZoneDataStore)
	if !ok {
		errMsg := fmt.Sprintf("Zone data for uri %s not found", uri)
		log.Error(errMsg)
		resp := updateErrorResponse(response.ResourceNotFound, errMsg, []interface{}{"Zone", uri})
		ctx.StatusCode(http.StatusNotFound)
		ctx.JSON(resp)
		return
	}
	if respData.Zone.Links != nil {
		if respData.Zone.Links.ContainsZonesCount != 0 {
			errMsg := fmt.Sprintf("Zone cannot be deleted as there are dependent resources still tied to it")
			log.Error(errMsg)
			resp := updateErrorResponse(response.ResourceCannotBeDeleted, errMsg, []interface{}{"Zone", uri})
			ctx.StatusCode(http.StatusNotAcceptable)
			ctx.JSON(resp)
			return
		}
	}
	if respData.Zone.ZoneType == "ZoneOfZones" {
		err := deleteZoneOfZone(respData, uri)
		if err != nil {
			errMsg := fmt.Sprintf("Zone data for uri %s not found", uri)
			log.Error(errMsg)
			resp := updateErrorResponse(response.ResourceNotFound, errMsg, []interface{}{"Zone", uri})
			ctx.StatusCode(http.StatusNotFound)
			ctx.JSON(resp)
			return
		}
		delete(capdata.ZoneDataStore, uri)
		ctx.JSON(http.StatusNoContent)
	}
	if respData.Zone.ZoneType == "Default" {
		aciClient := caputilities.GetConnection()
		err := aciClient.DeleteTenant(respData.Zone.Name)
		if err != nil {
			errMsg := "Error while deleting Zone: " + err.Error()
			resp := updateErrorResponse(response.GeneralError, errMsg, nil)
			ctx.StatusCode(http.StatusBadRequest)
			ctx.JSON(resp)
			return
		}

		delete(capdata.ZoneDataStore, uri)
		ctx.JSON(http.StatusNoContent)
	}

}

func deleteZoneOfZone(respData *capdata.ZoneData, uri string) error {
	var parentZoneLink model.Link
	if respData.Zone.Links != nil {
		if respData.Zone.Links.ContainedByZonesCount != 0 {
			// Assuming contained by link is only one
			parentZoneLink = respData.Zone.Links.ContainedByZones[0]
			parentZoneData, ok := capdata.ZoneDataStore[parentZoneLink.Oid]
			if !ok {
				errMsg := fmt.Errorf("Zone data for uri %s not found", uri)
				return errMsg
			}
			parentZone := parentZoneData.Zone
			links := parentZone.Links.ContainsZones
			var parentZoneIndex int
			for index, value := range links {
				if value.Oid == uri {
					parentZoneIndex = index
					break
				}
			}
			parentZone.Links.ContainsZones = append(links[:parentZoneIndex], links[parentZoneIndex+1:]...)
			parentZone.Links.ContainsZonesCount = len(parentZone.Links.ContainsZones)
			parentZoneData.Zone = parentZone
			capdata.ZoneDataStore[parentZoneLink.Oid] = parentZoneData
		}
		delete(capdata.ZoneDataStore, uri)
		return nil
	}
	return nil
}

// CreateZoneOfZones takes the request to create zone of zones and translates to create application profiles and VRFs
func CreateZoneOfZones(uri string, fabricID string, zone model.Zone) (string, interface{}, int) {
	var apModel aciModels.ApplicationProfileAttributes
	var vrfModel aciModels.VRFAttributes
	apModel.Name = zone.Name
	vrfModel.Name = zone.Name + "-VRF"
	if zone.Links != nil {
		if len(zone.Links.ContainedByZones) == 0 {
			errMsg := fmt.Sprintf("Zone cannot be created as there are dependent resources missing")
			log.Error(errMsg)
			resp := updateErrorResponse(response.PropertyMissing, errMsg, []interface{}{"ContainedByZones"})
			return "", resp, http.StatusBadRequest
		}
	}
	log.Println("Request Body")
	log.Println(zone)
	// Assuming there is only link under ContainedByZones
	defaultZoneLinks := zone.Links.ContainedByZones
	defaultZoneLink := defaultZoneLinks[0].Oid
	respData, ok := capdata.ZoneDataStore[defaultZoneLink]
	if !ok {
		errMsg := fmt.Sprintf("Zone data for uri %s not found", defaultZoneLink)
		log.Error(errMsg)
		resp := updateErrorResponse(response.ResourceNotFound, errMsg, []interface{}{"Zone", defaultZoneLink})
		return "", resp, http.StatusNotFound
	}
	aciClient := caputilities.GetConnection()
	appProfileList, err := aciClient.ListApplicationProfile(respData.Zone.Name)
	if err != nil && !strings.Contains(err.Error(), "Object may not exists") {
		errMsg := fmt.Sprintf("Zone cannot be created, error while retriving existing Application profiles: " + err.Error())
		log.Error(respData.Zone.Name)
		log.Error(errMsg)
		resp := updateErrorResponse(response.PropertyMissing, errMsg, []interface{}{"ContainedByZones"})
		return "", resp, http.StatusBadRequest
	}
	for _, appProfile := range appProfileList {
		if appProfile.ApplicationProfileAttributes.Name == zone.Name {
			errMsg := "Application profile already exists with name: " + zone.Name
			resp := updateErrorResponse(response.ResourceAlreadyExists, errMsg, []interface{}{"ApplicationProfile", appProfile.ApplicationProfileAttributes.Name, zone.Name})
			return "", resp, http.StatusConflict
		}
	}
	vrfList, err := aciClient.ListVRF(respData.Zone.Name)
	if err != nil && !strings.Contains(err.Error(), "Object may not exists") {
		errMsg := fmt.Sprintf("Zone cannot be created, error while retriving existing VRFs: " + err.Error())
		log.Error(errMsg)
		resp := updateErrorResponse(response.PropertyMissing, errMsg, []interface{}{"ContainedByZones"})
		return "", resp, http.StatusBadRequest
	}
	for _, vrf := range vrfList {
		if vrf.VRFAttributes.Name == vrfModel.Name {
			errMsg := "VRF already exists with name: " + vrfModel.Name
			resp := updateErrorResponse(response.ResourceAlreadyExists, errMsg, []interface{}{"VRF", vrf.VRFAttributes.Name, vrfModel.Name})
			return "", resp, http.StatusConflict
		}
	}

	apResp, err := CreateApplicationProfile(zone.Name, respData.Zone.Name, respData.Zone.Description, apModel)
	if err != nil {
		errMsg := "Error while creating application profile: " + err.Error()
		resp := updateErrorResponse(response.GeneralError, errMsg, nil)
		return "", resp, http.StatusBadRequest
	}
	_, vrfErr := CreateVRF(vrfModel.Name, respData.Zone.Name, respData.Zone.Description, vrfModel)
	if vrfErr != nil {
		errMsg := "Error while creating application profile: " + vrfErr.Error()
		resp := updateErrorResponse(response.GeneralError, errMsg, nil)
		return "", resp, http.StatusBadRequest
	}
	return defaultZoneLink, apResp, http.StatusCreated

}

func updateZoneData(defaultZoneLink string, zone model.Zone) {
	defaultZoneStore := capdata.ZoneDataStore[defaultZoneLink]
	defaultZoneData := defaultZoneStore.Zone
	if defaultZoneData.Links == nil {
		defaultZoneData.Links = &model.ZoneLinks{}
	}
	if defaultZoneData.Links.ContainsZones == nil {
		var containsList []model.Link
		log.Println("List of contains")
		log.Println(containsList)
		var link model.Link
		link.Oid = zone.ODataID
		containsList = append(containsList, link)
		defaultZoneData.Links.ContainsZones = containsList
		defaultZoneData.Links.ContainsZonesCount = len(containsList)
	} else {
		var link model.Link
		link.Oid = zone.ODataID
		defaultZoneData.Links.ContainsZones = append(defaultZoneData.Links.ContainsZones, link)
	}

	capdata.ZoneDataStore[defaultZoneLink].Zone = defaultZoneData
	return
}

func createZoneOfEndpoints(uri, fabricID string, zone model.Zone) (interface{}, int) {
	// Create the BridgeDomain
	// get the Tenant name from the ZoneofZone data
	//validate the request
	if zone.Links == nil {
		errorMessage := "Links attribute is missing in the request"
		return updateErrorResponse(response.PropertyMissing, errorMessage, []interface{}{"Links"}), http.StatusBadRequest
	}
	if zone.Links.ContainedByZones == nil {
		errorMessage := "ContainedByZones attribute is missing in the request"
		return updateErrorResponse(response.PropertyMissing, errorMessage, []interface{}{"ContainedByZones"}), http.StatusBadRequest

	}
	zoneofZoneURL := zone.Links.ContainedByZones[0].Oid
	// get the zone of zone data
	zoneofZoneData, ok := capdata.ZoneDataStore[zoneofZoneURL]
	if !ok {
		errMsg := fmt.Sprintf("ZoneofZone data for uri %s not found", uri)
		log.Error(errMsg)
		return updateErrorResponse(response.ResourceNotFound, errMsg, []interface{}{"ZoneofZone", zoneofZoneURL}), http.StatusNotFound
	}
	// Get the default zone data
	defaultZoneURL := zoneofZoneData.Zone.Links.ContainedByZones[0].Oid
	defaultZoneData := capdata.ZoneDataStore[defaultZoneURL]
	return createBridgeDomain(defaultZoneData.Zone.Name, zone)
}

func createBridgeDomain(tenantName string, zone model.Zone) (interface{}, int) {
	var bridgeDomainAttributes aciModels.BridgeDomainAttributes
	bridgeDomainAttributes.Name = zone.Name
	aciClient := caputilities.GetConnection()
	//var tenantList []*aciModels.Tenant
	bridgeDomainList, err := aciClient.ListBridgeDomain(tenantName)
	if err != nil && !strings.Contains(err.Error(), "Object may not exists") {
		errMsg := "Error while creating Zone endpoints: " + err.Error()
		log.Error(errMsg)
		resp := updateErrorResponse(response.GeneralError, errMsg, nil)
		return resp, http.StatusBadRequest
	}
	for _, bd := range bridgeDomainList {
		if bd.Name == zone.Name {
			errMsg := "ZoneOfEndpoints already exists with name: " + zone.Name + " for the default zone " + tenantName
			resp := updateErrorResponse(response.ResourceAlreadyExists, errMsg, []interface{}{"ZoneOfEndpoints", bd.BridgeDomainAttributes.Name, zone.Name})
			return resp, http.StatusConflict
		}

	}

	resp, err := aciClient.CreateBridgeDomain(zone.Name, tenantName, zone.Description, bridgeDomainAttributes)
	if err != nil {
		errMsg := "Error while creating  Zone of Endpoints: " + err.Error()
		resp := updateErrorResponse(response.GeneralError, errMsg, nil)
		return resp, http.StatusBadRequest
	}
	return resp, http.StatusCreated
}