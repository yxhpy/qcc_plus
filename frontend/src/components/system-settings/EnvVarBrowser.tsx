import { useDeferredValue, useEffect, useMemo, useState } from "react";
import Card from "../Card";
import api from "../../services/api";
import type { EnvVarCategory, EnvVarDefinition } from "../../services/api";

export default function EnvVarBrowser() {
  const [categories, setCategories] = useState<EnvVarCategory[]>([]);
  const [envvars, setEnvvars] = useState<EnvVarDefinition[]>([]);
  const [activeCategory, setActiveCategory] = useState("");
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState("");
  const deferredSearchTerm = useDeferredValue(searchTerm);

  useEffect(() => {
    const loadCategories = async () => {
      try {
        const data = await api.getEnvVarCategories();
        setCategories(data);
        if (data.length > 0) {
          setActiveCategory(data[0].key);
        }
      } catch (err) {
        console.error("Failed to load categories:", err);
      }
    };
    loadCategories();
  }, []);

  useEffect(() => {
    if (!activeCategory) return;
    const loadEnvVars = async () => {
      setLoading(true);
      try {
        const vars = await api.getEnvVars(activeCategory);
        setEnvvars(vars);
      } catch (err) {
        console.error("Failed to load env vars:", err);
      } finally {
        setLoading(false);
      }
    };
    loadEnvVars();
  }, [activeCategory]);

  const filteredEnvvars = useMemo(() => {
    if (!deferredSearchTerm) {
      return envvars;
    }
    const term = deferredSearchTerm.toLowerCase();
    return envvars.filter(
      (item) =>
        item.name.toLowerCase().includes(term) ||
        item.description.toLowerCase().includes(term) ||
        item.current_value.toLowerCase().includes(term),
    );
  }, [deferredSearchTerm, envvars]);

  const currentCategoryInfo = categories.find(
    (item) => item.key === activeCategory,
  );

  return (
    <section className="settings-section">
      <div className="section-heading">
        <h2>环境变量</h2>
        <p>
          这里展示启动时读取的配置来源。修改环境变量后仍需重启服务，适合部署层面的固定参数管理。
        </p>
      </div>

      <Card className="settings-card tabs-card">
        <div className="settings-toolbar">
          <div className="tab-group">
            {categories.map((cat) => (
              <button
                key={cat.key}
                type="button"
                className={`tab-btn ${activeCategory === cat.key ? "active" : ""}`}
                onClick={() => setActiveCategory(cat.key)}
                title={cat.description}
              >
                {cat.label}
              </button>
            ))}
          </div>
          <div className="spacer" />
          <div className="search-box">
            <input
              type="text"
              placeholder="搜索变量名或说明..."
              value={searchTerm}
              onChange={(event) => setSearchTerm(event.target.value)}
            />
            {searchTerm && (
              <button
                type="button"
                className="clear-btn"
                onClick={() => setSearchTerm("")}
              >
                &times;
              </button>
            )}
          </div>
        </div>
      </Card>

      {currentCategoryInfo && (
        <div className="category-description">
          {currentCategoryInfo.description}
        </div>
      )}

      <Card className="settings-card">
        <div className="envvar-table-container">
          {loading ? (
            <div className="settings-loading">加载中...</div>
          ) : filteredEnvvars.length === 0 ? (
            <div className="no-settings">
              {searchTerm ? "没有匹配的环境变量" : "该分类暂无环境变量"}
            </div>
          ) : (
            <table className="envvar-table">
              <thead>
                <tr>
                  <th className="col-name">变量名</th>
                  <th className="col-value">当前值</th>
                  <th className="col-default">默认值</th>
                  <th className="col-desc">说明</th>
                </tr>
              </thead>
              <tbody>
                {filteredEnvvars.map((item) => (
                  <tr key={item.name} className={item.is_set ? "is-set" : ""}>
                    <td className="col-name">
                      <code className="envvar-name">{item.name}</code>
                      {item.is_set && <span className="set-badge">已设置</span>}
                    </td>
                    <td className="col-value">
                      {item.is_secret ? (
                        <span className="secret-value">
                          {item.current_value || "(未设置)"}
                        </span>
                      ) : (
                        <code
                          className={`envvar-value ${!item.current_value ? "empty" : ""}`}
                        >
                          {item.current_value || "(空)"}
                        </code>
                      )}
                    </td>
                    <td className="col-default">
                      <code
                        className={`default-value ${!item.default_value ? "empty" : ""}`}
                      >
                        {item.default_value || "(无)"}
                      </code>
                    </td>
                    <td className="col-desc">
                      <span className="envvar-desc">{item.description}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </Card>

      <Card className="settings-card info-card">
        <div className="info-section">
          <h3>使用说明</h3>
          <ul>
            <li>
              <strong>修改环境变量</strong>：编辑 <code>.env</code> 或 Docker
              Compose 中的环境变量，然后重启服务
            </li>
            <li>
              <strong>敏感值</strong>：API Key 等敏感信息会脱敏显示
            </li>
            <li>
              <strong>已设置标记</strong>：表示该变量已在当前部署环境中显式配置
            </li>
            <li>
              <strong>职责边界</strong>
              ：运行时可热改的参数请在上方“运行时配置”中维护
            </li>
          </ul>
        </div>
      </Card>
    </section>
  );
}
