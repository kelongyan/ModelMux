import { useMutation, useQuery } from "@tanstack/react-query";
import { Button, Card, Result, Skeleton, Space, Typography, message } from "antd";
import { DownloadOutlined, FileProtectOutlined, CloudServerOutlined, BranchesOutlined } from "@ant-design/icons";

import { downloadConfigBackup, downloadStateBackup, fetchAbout } from "../api/admin";
import { queryKeys } from "../api/query-keys";

export function AboutPage(): JSX.Element {
  const [messageApi, contextHolder] = message.useMessage();
  const aboutQuery = useQuery({
    queryKey: queryKeys.about,
    queryFn: fetchAbout,
  });

  const configBackupMutation = useMutation({
    mutationFn: downloadConfigBackup,
    onSuccess: () => messageApi.success("配置备份已开始下载"),
    onError: (error: Error) => messageApi.error(`配置备份失败：${error.message}`),
  });

  const stateBackupMutation = useMutation({
    mutationFn: downloadStateBackup,
    onSuccess: () => messageApi.success("状态备份已开始下载"),
    onError: (error: Error) => messageApi.error(`状态备份失败：${error.message}`),
  });

  if (aboutQuery.isLoading) {
    return (
      <div className="console-loading">
        <Skeleton active paragraph={{ rows: 8 }} />
      </div>
    );
  }

  if (aboutQuery.isError || !aboutQuery.data) {
    return (
      <Result
        status="error"
        title="关于页加载失败"
        subTitle={aboutQuery.error instanceof Error ? aboutQuery.error.message : "未知错误"}
        extra={<Button onClick={() => void aboutQuery.refetch()}>重试</Button>}
      />
    );
  }

  const about = aboutQuery.data;

  return (
    <>
      {contextHolder}
      <Space direction="vertical" size={16} className="console-stack">
        {/* ── Hero ── */}
        <Card className="surface-card about-hero-card reveal-card reveal-delay-0" bordered={false}>
          <div className="about-hero">
            <div className="about-hero-left">
              <span className="about-hero-kicker">About</span>
              <h2 className="about-hero-title">{about.app_name || "ModelMux"}</h2>
              <div className="about-hero-tags">
                <span className="about-tag about-tag--version">{about.version}</span>
                <span className="about-tag about-tag--platform">{about.platform}</span>
                <span className="about-tag about-tag--go">{about.go_version}</span>
              </div>
            </div>
            <div className="about-hero-right">
              <span className="about-hero-build">{about.build_time || "—"}</span>
            </div>
          </div>
        </Card>

        {/* ── Info grid ── */}
        <div className="about-grid">
          <Card className="surface-card reveal-card reveal-delay-1" bordered={false}>
            <div className="about-section-head">
              <CloudServerOutlined className="about-section-icon" />
              <span className="about-section-label">网络与服务</span>
            </div>
            <div className="about-rows">
              <InfoRow label="代理监听" value={about.listen} mono />
              <InfoRow label="管理监听" value={about.admin_listen} mono />
              <InfoRow label="活跃 Provider" value={about.active_provider || "未配置"} />
              <InfoRow label="Provider 数量" value={String(about.provider_count)} />
            </div>
          </Card>

          <Card className="surface-card reveal-card reveal-delay-2" bordered={false}>
            <div className="about-section-head">
              <BranchesOutlined className="about-section-icon" />
              <span className="about-section-label">配置与状态</span>
            </div>
            <div className="about-rows">
              <InfoRow label="配置文件" value={about.config_path} mono truncate />
              <InfoRow label="状态文件" value={about.state_file} mono truncate />
              <InfoRow label="构建时间" value={about.build_time || "—"} />
            </div>
          </Card>
        </div>

        {/* ── Features ── */}
        {about.features.length > 0 && (
          <Card className="surface-card reveal-card reveal-delay-2" bordered={false}>
            <div className="about-section-head">
              <FileProtectOutlined className="about-section-icon" />
              <span className="about-section-label">已启用能力</span>
            </div>
            <div className="about-features">
              <Typography.Text className="about-features-text">
                {about.features.join("、")}
              </Typography.Text>
            </div>
          </Card>
        )}

        {/* ── Backup actions ── */}
        <Card className="surface-card reveal-card reveal-delay-3" bordered={false}>
          <div className="about-section-head">
            <DownloadOutlined className="about-section-icon" />
            <span className="about-section-label">导出备份</span>
          </div>
          <div className="about-backup-row">
            <div className="about-backup-desc">
              <strong>配置备份</strong>
              <span>导出当前 config.json，包含所有 Provider 和运行参数。</span>
            </div>
            <Button
              size="small"
              type="primary"
              icon={<DownloadOutlined />}
              loading={configBackupMutation.isPending}
              onClick={() => configBackupMutation.mutate()}
            >
              下载配置
            </Button>
          </div>
          <div className="about-backup-divider" />
          <div className="about-backup-row">
            <div className="about-backup-desc">
              <strong>状态备份</strong>
              <span>导出 Key 池状态快照（仅含哈希，不含明文密钥）。</span>
            </div>
            <Button
              size="small"
              icon={<DownloadOutlined />}
              loading={stateBackupMutation.isPending}
              onClick={() => stateBackupMutation.mutate()}
            >
              下载状态
            </Button>
          </div>
        </Card>
      </Space>
    </>
  );
}

function InfoRow({ label, value, mono, truncate }: {
  label: string;
  value: string;
  mono?: boolean;
  truncate?: boolean;
}): JSX.Element {
  const valClass = `about-val${mono ? " about-val--mono" : ""}${truncate ? " about-val--truncate" : ""}`;
  return (
    <div className="about-row">
      <span className="about-label">{label}</span>
      <span className={valClass} title={truncate ? value : undefined}>{value}</span>
    </div>
  );
}
