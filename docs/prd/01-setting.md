完成设置页面的功能
功能清单：
点击重置默认，读取当前项目下的配置。现在配置有两个地方：backend/agent-frame/manifest/config
backend/runner/skills-config.yaml

你给出建议，这种怎么设计好呢？
还是增加两个配置输入框呢？

另外当前项目是按照yaml做的，是不是json更利于用户来做呢？

1.前端页面：Settings.tsx
2.后端业务代码都在：backend/agent-frame中实现；agent-frame规范参考：agent-frame/skills.md