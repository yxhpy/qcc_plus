import { useState } from "react";
import Card from "../components/Card";
import Toast from "../components/Toast";
import EnvVarBrowser from "../components/system-settings/EnvVarBrowser";
import RuntimeSettingsPanel from "../components/system-settings/RuntimeSettingsPanel";
import "./SystemSettings.css";

export default function SystemSettings() {
  const [toast, setToast] = useState<{
    message: string;
    type: "success" | "error";
  } | null>(null);

  const showToast = (
    message: string,
    type: "success" | "error" = "success",
  ) => {
    setToast({ message, type });
    window.setTimeout(() => setToast(null), 2400);
  };

  return (
    <div className="system-settings-page">
      <div className="system-settings-header">
        <h1>系统设置</h1>
        <p className="sub">
          系统级运行时参数现在支持前端维护、数据库持久化和热更新。需要重启的部署参数仍保留在下方环境变量区，只读展示。
        </p>
      </div>

      <div className="settings-overview-grid">
        <Card className="settings-card overview-card">
          <span className="overview-kicker">运行时配置</span>
          <h3>保存即写库</h3>
          <p>
            可编辑配置会写入 <code>settings</code>{" "}
            表，重启后仍能恢复，不再依赖临时内存状态。
          </p>
        </Card>
        <Card className="settings-card overview-card">
          <span className="overview-kicker">热更新</span>
          <h3>当前实例即时生效</h3>
          <p>
            支持热更新的后端参数会通过 <code>SettingsCache</code>{" "}
            回调推送；前端配置会在当前标签页立即更新。
          </p>
        </Card>
        <Card className="settings-card overview-card">
          <span className="overview-kicker">环境变量</span>
          <h3>仍归部署层管理</h3>
          <p>
            只在服务启动时读取的参数继续通过环境变量控制，页面只负责清晰展示，不伪装成可热改。
          </p>
        </Card>
      </div>

      <RuntimeSettingsPanel onToast={showToast} />
      <EnvVarBrowser />

      <Toast message={toast?.message} type={toast?.type} />
    </div>
  );
}
