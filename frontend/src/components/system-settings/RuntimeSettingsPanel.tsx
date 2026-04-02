import { useEffect, useMemo, useState } from "react";
import Card from "../Card";
import { useSettings } from "../../contexts/SettingsContext";
import {
  settingsApi,
  type RuntimeSettingDefinition,
} from "../../services/settingsApi";

type DraftValue = string | boolean;

interface RuntimeSettingsPanelProps {
  onToast: (message: string, type?: "success" | "error") => void;
}

function toDraftValue(def: RuntimeSettingDefinition, value: any): DraftValue {
  if (def.data_type === "boolean") {
    return Boolean(value);
  }
  return String(value ?? "");
}

function parseDraftValue(
  def: RuntimeSettingDefinition,
  draft: DraftValue,
): number | boolean | null {
  if (def.data_type === "boolean") {
    return Boolean(draft);
  }
  const text = String(draft).trim();
  if (!text) {
    return null;
  }
  const parsed = Number(text);
  if (!Number.isFinite(parsed)) {
    return null;
  }
  return parsed;
}

function getDraftError(
  def: RuntimeSettingDefinition,
  draft: DraftValue,
): string | null {
  const value = parseDraftValue(def, draft);
  if (value === null) {
    return "请输入有效值";
  }
  if (typeof value !== "number") {
    return null;
  }
  if (typeof def.min === "number" && value < def.min) {
    return `不能小于 ${def.min}`;
  }
  if (typeof def.max === "number" && value > def.max) {
    return `不能大于 ${def.max}`;
  }
  return null;
}

function isSameValue(a: number | boolean | null, b: any): boolean {
  if (typeof a === "boolean") {
    return a === Boolean(b);
  }
  if (typeof a === "number") {
    return a === Number(b);
  }
  return false;
}

function formatValue(def: RuntimeSettingDefinition, value: any): string {
  if (def.data_type === "boolean") {
    return Boolean(value) ? "开启" : "关闭";
  }
  return def.unit ? `${value}${def.unit}` : String(value);
}

export default function RuntimeSettingsPanel({
  onToast,
}: RuntimeSettingsPanelProps) {
  const { settings, loading, error, refresh } = useSettings();
  const [definitions, setDefinitions] = useState<RuntimeSettingDefinition[]>(
    [],
  );
  const [catalogLoading, setCatalogLoading] = useState(true);
  const [catalogError, setCatalogError] = useState("");
  const [drafts, setDrafts] = useState<Record<string, DraftValue>>({});
  const [savingKey, setSavingKey] = useState("");

  useEffect(() => {
    const loadDefinitions = async () => {
      setCatalogLoading(true);
      try {
        const data = await settingsApi.getRuntimeDefinitions();
        setDefinitions(data);
        setCatalogError("");
      } catch (err) {
        setCatalogError((err as Error).message || "加载运行时配置目录失败");
      } finally {
        setCatalogLoading(false);
      }
    };
    loadDefinitions();
  }, []);

  const groups = useMemo(() => {
    const map = new Map<
      string,
      { label: string; items: RuntimeSettingDefinition[] }
    >();
    definitions.forEach((def) => {
      const group = map.get(def.category) ?? {
        label: def.category_label,
        items: [],
      };
      group.items.push(def);
      map.set(def.category, group);
    });
    return Array.from(map.entries()).map(([key, value]) => ({ key, ...value }));
  }, [definitions]);

  const setDraft = (key: string, value: DraftValue) => {
    setDrafts((prev) => ({ ...prev, [key]: value }));
  };

  const clearDraft = (key: string) => {
    setDrafts((prev) => {
      if (!(key in prev)) return prev;
      const next = { ...prev };
      delete next[key];
      return next;
    });
  };

  const handleSave = async (def: RuntimeSettingDefinition) => {
    const currentSetting = settings[def.key];
    const currentValue = currentSetting?.value ?? def.default_value;
    const draft = drafts[def.key] ?? toDraftValue(def, currentValue);
    const parsedValue = parseDraftValue(def, draft);
    const validationError = getDraftError(def, draft);
    if (parsedValue === null || validationError) {
      onToast(validationError || `保存 ${def.label} 失败`, "error");
      return;
    }

    setSavingKey(def.key);
    try {
      await settingsApi.save(def.key, {
        value: parsedValue,
        version: currentSetting?.version || 0,
        scope: "system",
        data_type: def.data_type,
        category: def.category,
        description: def.description,
      });
      clearDraft(def.key);
      await refresh();
      onToast(`${def.label} 已保存`);
    } catch (err) {
      const message = (err as Error).message || "保存失败";
      if (message === "version_conflict") {
        await refresh();
        onToast(`${def.label} 已被其他操作更新，页面已刷新，请重试`, "error");
      } else {
        onToast(message, "error");
      }
    } finally {
      setSavingKey("");
    }
  };

  return (
    <section className="settings-section">
      <div className="section-heading">
        <h2>运行时配置</h2>
        <p>
          这些配置会写入数据库持久化保存。支持热更新的参数保存后立即应用到当前实例。
        </p>
      </div>

      <Card className="settings-card runtime-summary-card">
        <div className="runtime-summary-grid">
          <div className="runtime-summary-item">
            <span className="runtime-summary-label">当前实例</span>
            <strong>保存后立即生效</strong>
          </div>
          <div className="runtime-summary-item">
            <span className="runtime-summary-label">多实例同步</span>
            <strong>依赖 settings watcher 轮询</strong>
          </div>
          <div className="runtime-summary-item">
            <span className="runtime-summary-label">持久化</span>
            <strong>写入 settings 表</strong>
          </div>
        </div>
      </Card>

      {(catalogLoading || loading) && (
        <Card className="settings-card">
          <div className="settings-loading">加载中...</div>
        </Card>
      )}

      {!catalogLoading && catalogError && (
        <Card className="settings-card">
          <div className="settings-error">{catalogError}</div>
        </Card>
      )}
      {!loading && error && (
        <Card className="settings-card">
          <div className="settings-error">{error}</div>
        </Card>
      )}

      {!catalogLoading &&
        !loading &&
        !catalogError &&
        !error &&
        groups.map((group) => (
          <Card key={group.key} className="settings-card" title={group.label}>
            <div className="runtime-setting-grid">
              {group.items.map((def) => {
                const currentSetting = settings[def.key];
                const currentValue = currentSetting?.value ?? def.default_value;
                const draft =
                  drafts[def.key] ?? toDraftValue(def, currentValue);
                const parsedDraft = parseDraftValue(def, draft);
                const validationError = getDraftError(def, draft);
                const dirty =
                  parsedDraft !== null &&
                  !isSameValue(parsedDraft, currentValue);
                const busy = savingKey === def.key;

                return (
                  <article key={def.key} className="runtime-setting-card">
                    <div className="runtime-setting-head">
                      <div>
                        <h3>{def.label}</h3>
                        <code>{def.key}</code>
                      </div>
                      <div className="setting-badges">
                        {def.persisted && (
                          <span className="setting-badge persisted">
                            已持久化
                          </span>
                        )}
                        {def.hot_reload && (
                          <span className="setting-badge hot">热更新</span>
                        )}
                        <span className="setting-badge mode">
                          {def.apply_mode_label}
                        </span>
                      </div>
                    </div>

                    <p className="runtime-setting-desc">{def.description}</p>

                    <div className="runtime-setting-meta">
                      <span>当前值：{formatValue(def, currentValue)}</span>
                      <span>默认值：{formatValue(def, def.default_value)}</span>
                    </div>
                    <div className="runtime-setting-note">{def.sync_note}</div>

                    <div className="runtime-setting-editor">
                      {def.data_type === "boolean" ? (
                        <label className="switch-field">
                          <input
                            type="checkbox"
                            checked={Boolean(draft)}
                            onChange={(event) =>
                              setDraft(def.key, event.target.checked)
                            }
                          />
                          <span>{Boolean(draft) ? "开启" : "关闭"}</span>
                        </label>
                      ) : (
                        <label className="number-field">
                          <input
                            type="number"
                            value={String(draft)}
                            min={def.min}
                            max={def.max}
                            step={def.step || 1}
                            onChange={(event) =>
                              setDraft(def.key, event.target.value)
                            }
                          />
                          {def.unit && (
                            <span className="field-unit">{def.unit}</span>
                          )}
                        </label>
                      )}
                      <div className="runtime-setting-actions">
                        <button
                          type="button"
                          className="btn primary"
                          disabled={busy || !dirty || Boolean(validationError)}
                          onClick={() => handleSave(def)}
                        >
                          {busy ? "保存中..." : "保存"}
                        </button>
                        <button
                          type="button"
                          className="btn ghost"
                          disabled={busy || !dirty}
                          onClick={() => clearDraft(def.key)}
                        >
                          重置
                        </button>
                      </div>
                    </div>

                    {validationError && (
                      <div className="runtime-setting-error">
                        {validationError}
                      </div>
                    )}
                  </article>
                );
              })}
            </div>
          </Card>
        ))}
    </section>
  );
}
