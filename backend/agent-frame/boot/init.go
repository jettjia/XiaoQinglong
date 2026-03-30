package boot

func InitData() (err error) {
	// 初始化扫描 skills 目录并同步到数据库
	if err = syncSkillsFromDisk(); err != nil {
		return
	}

	// 初始化默认渠道
	if err = initDefaultChannels(); err != nil {
		return
	}

	// 初始化默认智能体
	if err = initDefaultAgents(); err != nil {
		return
	}

	return
}

type InitHandler struct {
}

func NewInitHandler() *InitHandler {
	return &InitHandler{}
}
