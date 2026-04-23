package plugin

import (
	"context"

	"github.com/jettjia/igo-pkg/pkg/data"
	"github.com/jettjia/igo-pkg/pkg/xsql/builder"

	entityPlugin "github.com/jettjia/xiaoqinglong/agent-frame/domain/entity/plugin"
	irepositoryPlugin "github.com/jettjia/xiaoqinglong/agent-frame/domain/irepository/plugin"
	"github.com/jettjia/xiaoqinglong/agent-frame/infra/pkg/idata"
	converterPlugin "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/converter/plugin"
	poPlugin "github.com/jettjia/xiaoqinglong/agent-frame/infra/repository/po/plugin"
)

var _ irepositoryPlugin.IPluginRepo = (*SysPlugin)(nil)

type SysPlugin struct {
	data *data.Data
}

func NewSysPluginImpl() *SysPlugin {
	return &SysPlugin{
		data: idata.NewDataOptionCli(),
	}
}

// CreateInstance 创建插件实例
func (r *SysPlugin) CreateInstance(ctx context.Context, instanceEn *entityPlugin.PluginInstance) (ulid string, err error) {
	instancePo := converterPlugin.E2PInstanceAdd(instanceEn)
	if err = r.data.DB(ctx).Create(&instancePo).Error; err != nil {
		return
	}
	return instancePo.Ulid, nil
}

// DeleteInstance 删除插件实例
func (r *SysPlugin) DeleteInstance(ctx context.Context, instanceEn *entityPlugin.PluginInstance) (err error) {
	instancePo := converterPlugin.E2PInstanceDel(instanceEn)
	return r.data.DB(ctx).Model(&poPlugin.PluginInstancePO{}).Where("ulid = ?", instanceEn.Ulid).Updates(instancePo).Error
}

// UpdateInstance 更新插件实例
func (r *SysPlugin) UpdateInstance(ctx context.Context, instanceEn *entityPlugin.PluginInstance) (err error) {
	instancePo := converterPlugin.E2PInstanceUpdate(instanceEn)
	return r.data.DB(ctx).Model(&poPlugin.PluginInstancePO{}).Where("ulid = ?", instanceEn.Ulid).Updates(instancePo).Error
}

// FindInstanceById 根据ID查询实例
func (r *SysPlugin) FindInstanceById(ctx context.Context, ulid string) (instanceEn *entityPlugin.PluginInstance, err error) {
	var instancePo poPlugin.PluginInstancePO
	if err = r.data.DB(ctx).Limit(1).Find(&instancePo, "ulid = ?", ulid).Error; err != nil {
		return
	}
	instanceEn = converterPlugin.P2EInstance(&instancePo)
	return
}

// FindInstanceByQuery 根据条件查询单个实例
func (r *SysPlugin) FindInstanceByQuery(ctx context.Context, queries []*builder.Query) (instanceEn *entityPlugin.PluginInstance, err error) {
	whereStr, values, err := builder.GormBuildWhere(queries)
	if err != nil {
		return
	}
	var instancePo poPlugin.PluginInstancePO
	if err = r.data.DB(ctx).Model(&poPlugin.PluginInstancePO{}).Limit(1).Where(whereStr, values...).Find(&instancePo).Error; err != nil {
		return
	}
	instanceEn = converterPlugin.P2EInstance(&instancePo)
	return
}

// FindInstanceByUserAndPlugin 根据用户ID和插件ID查询实例
func (r *SysPlugin) FindInstanceByUserAndPlugin(ctx context.Context, userID, pluginID string) (instanceEn *entityPlugin.PluginInstance, err error) {
	var instancePo poPlugin.PluginInstancePO
	if err = r.data.DB(ctx).Limit(1).Where("user_id = ? AND plugin_id = ?", userID, pluginID).Find(&instancePo).Error; err != nil {
		return
	}
	instanceEn = converterPlugin.P2EInstance(&instancePo)
	return
}

// FindAllInstance 查询用户所有插件实例
func (r *SysPlugin) FindAllInstance(ctx context.Context, queries []*builder.Query) (entries []*entityPlugin.PluginInstance, err error) {
	whereStr, values, err := builder.GormBuildWhere(queries)
	if err != nil {
		return
	}
	instancePos := make([]*poPlugin.PluginInstancePO, 0)
	if err = r.data.DB(ctx).Model(&poPlugin.PluginInstancePO{}).Where(whereStr, values...).Order("ulid desc").Find(&instancePos).Error; err != nil {
		return
	}
	entries = converterPlugin.P2EInstances(instancePos)
	return
}

// FindPageInstance 分页查询插件实例
func (r *SysPlugin) FindPageInstance(ctx context.Context, queries []*builder.Query, reqPage *builder.PageData, reqSort *builder.SortData) ([]*entityPlugin.PluginInstance, *builder.PageData, error) {
	var total int64
	instancePos := make([]*poPlugin.PluginInstancePO, 0)

	whereStr, values, err := builder.GormBuildWhere(queries)
	if err != nil {
		return nil, nil, err
	}

	// default reqSort
	if reqSort == nil {
		reqSort = &builder.SortData{Sort: "ulid", Direction: "desc"}
	}
	// default reqPage
	if reqPage == nil {
		reqPage = &builder.PageData{PageNum: 1, PageSize: 10}
	}

	dbQuery := r.data.DB(ctx).Model(&poPlugin.PluginInstancePO{}).Where(whereStr, values...)

	if err = dbQuery.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	rspPag := &builder.PageData{
		PageNum:     reqPage.PageNum,
		PageSize:    reqPage.PageSize,
		TotalNumber: total,
		TotalPage:   builder.CeilPageNum(total, reqPage.PageSize),
	}

	if total == 0 {
		return make([]*entityPlugin.PluginInstance, 0), rspPag, nil
	}

	err = dbQuery.
		Order(reqSort.Sort + " " + reqSort.Direction).
		Scopes(builder.GormPaginate(reqPage.PageNum, reqPage.PageSize)).
		Find(&instancePos).Error

	if err != nil {
		return nil, nil, err
	}

	entries := converterPlugin.P2EInstances(instancePos)
	return entries, rspPag, nil
}

// CreateOAuthState 创建OAuth状态
func (r *SysPlugin) CreateOAuthState(ctx context.Context, stateEn *entityPlugin.OAuthState) (err error) {
	statePo := converterPlugin.E2POAuthStateAdd(stateEn)
	return r.data.DB(ctx).Create(&statePo).Error
}

// DeleteOAuthState 删除OAuth状态
func (r *SysPlugin) DeleteOAuthState(ctx context.Context, state string) (err error) {
	return r.data.DB(ctx).Delete(&poPlugin.OAuthStatePO{}, "state = ?", state).Error
}

// FindOAuthStateByState 根据state查询OAuth状态
func (r *SysPlugin) FindOAuthStateByState(ctx context.Context, state string) (stateEn *entityPlugin.OAuthState, err error) {
	var statePo poPlugin.OAuthStatePO
	if err = r.data.DB(ctx).Limit(1).Find(&statePo, "state = ?", state).Error; err != nil {
		return
	}
	stateEn = converterPlugin.P2EOAuthState(&statePo)
	return
}