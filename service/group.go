package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type GroupInfo struct {
	Ratio      interface{} `json:"ratio"`
	Desc       string      `json:"desc"`
	Selectable bool        `json:"selectable"`
	AdminOnly  bool        `json:"admin_only"`
}

func applySpecialUsableGroups(groups map[string]string, userGroup string) map[string]string {
	if userGroup == "" {
		return groups
	}
	specialSettings, ok := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
	if !ok {
		return groups
	}
	for specialGroup, desc := range specialSettings {
		if strings.HasPrefix(specialGroup, "-:") {
			groupToRemove := strings.TrimPrefix(specialGroup, "-:")
			delete(groups, groupToRemove)
		} else if strings.HasPrefix(specialGroup, "+:") {
			groupToAdd := strings.TrimPrefix(specialGroup, "+:")
			groups[groupToAdd] = desc
		} else {
			groups[specialGroup] = desc
		}
	}
	return groups
}

func GetUserSelectableGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	return applySpecialUsableGroups(groupsCopy, userGroup)
}

func GetUserUsableGroups(userGroup string) map[string]string {
	groupsCopy := GetUserSelectableGroups(userGroup)
	if userGroup != "" {
		if _, ok := groupsCopy[userGroup]; !ok {
			groupsCopy[userGroup] = "用户分组"
		}
	}
	return groupsCopy
}

func GetConfiguredGroupInfos(userGroup string, includeAllConfigured bool) map[string]GroupInfo {
	selectableGroups := GetUserSelectableGroups(userGroup)
	infos := make(map[string]GroupInfo)
	if includeAllConfigured {
		for groupName := range ratio_setting.GetGroupRatioCopy() {
			desc, selectable := selectableGroups[groupName]
			if desc == "" {
				desc = setting.GetUsableGroupDescription(groupName)
			}
			infos[groupName] = GroupInfo{
				Ratio:      GetUserGroupRatio(userGroup, groupName),
				Desc:       desc,
				Selectable: selectable,
				AdminOnly:  !selectable,
			}
		}
		if len(GetUserAutoGroup(userGroup)) > 0 {
			desc, selectable := selectableGroups["auto"]
			if desc == "" {
				desc = setting.GetUsableGroupDescription("auto")
			}
			infos["auto"] = GroupInfo{
				Ratio:      "自动",
				Desc:       desc,
				Selectable: selectable,
				AdminOnly:  !selectable,
			}
		}
		return infos
	}

	for groupName, desc := range selectableGroups {
		if groupName == "auto" {
			if len(GetUserAutoGroup(userGroup)) == 0 {
				continue
			}
			infos[groupName] = GroupInfo{
				Ratio:      "自动",
				Desc:       desc,
				Selectable: true,
				AdminOnly:  false,
			}
			continue
		}
		if !ratio_setting.ContainsGroupRatio(groupName) {
			continue
		}
		infos[groupName] = GroupInfo{
			Ratio:      GetUserGroupRatio(userGroup, groupName),
			Desc:       desc,
			Selectable: true,
			AdminOnly:  false,
		}
	}
	return infos
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserSelectableGroups(userGroup)[groupName]
	return ok
}

func IsConfiguredRoutingGroup(group string) bool {
	if group == "" {
		return true
	}
	if group == "auto" {
		return len(GetUserAutoGroup("")) > 0
	}
	return ratio_setting.ContainsGroupRatio(group)
}

func CanSelectGroup(userGroup, groupName string, isAdmin bool) bool {
	if groupName == "" {
		return true
	}
	if isAdmin {
		return IsConfiguredRoutingGroup(groupName)
	}
	_, ok := GetUserSelectableGroups(userGroup)[groupName]
	if !ok {
		return false
	}
	if groupName == "auto" {
		return len(GetUserAutoGroup(userGroup)) > 0
	}
	return ratio_setting.ContainsGroupRatio(groupName)
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	autoGroups := make([]string, 0)
	for _, group := range setting.GetAutoGroups() {
		if ratio_setting.ContainsGroupRatio(group) {
			autoGroups = append(autoGroups, group)
		}
	}
	return autoGroups
}

// GetUserGroupRatio 获取用户使用某个分组的倍率
// userGroup 用户分组
// group 需要获取倍率的分组
func GetUserGroupRatio(userGroup, group string) float64 {
	ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group)
	if ok {
		return ratio
	}
	return ratio_setting.GetGroupRatio(group)
}

func GetVisibleGroupRatio(userGroup string, includeAllConfigured bool) map[string]float64 {
	groupRatio := make(map[string]float64)
	if includeAllConfigured {
		for group := range ratio_setting.GetGroupRatioCopy() {
			groupRatio[group] = GetUserGroupRatio(userGroup, group)
		}
		return groupRatio
	}
	for group := range GetUserSelectableGroups(userGroup) {
		if ratio_setting.ContainsGroupRatio(group) {
			groupRatio[group] = GetUserGroupRatio(userGroup, group)
		}
	}
	return groupRatio
}

func GetVisibleGroupNames(userGroup string, includeAllConfigured bool) map[string]string {
	groups := make(map[string]string)
	selectableGroups := GetUserSelectableGroups(userGroup)
	if includeAllConfigured {
		for group := range ratio_setting.GetGroupRatioCopy() {
			desc := selectableGroups[group]
			if desc == "" {
				desc = setting.GetUsableGroupDescription(group)
			}
			groups[group] = desc
		}
		return groups
	}
	for group, desc := range selectableGroups {
		if ratio_setting.ContainsGroupRatio(group) || (group == "auto" && len(GetUserAutoGroup(userGroup)) > 0) {
			groups[group] = desc
		}
	}
	return groups
}

func IsAdminRole(role int) bool {
	return role >= common.RoleAdminUser
}
