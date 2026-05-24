import { useMutation, useQuery } from "@tanstack/react-query";
import { Button, Card, Col, Descriptions, Result, Row, Space, Spin, Tag, Typography, message } from "antd";

import { downloadConfigBackup, downloadStateBackup, fetchAbout } from "../api/admin";

// AboutPage 展示运行环境信息，并提供配置和状态导出入口。
export function AboutPage(): JSX.Element {
  const [messageApi, contextHolder] = message.useMessage();
  const aboutQuery = useQuery({
    queryKey: ["about"],
    queryFn: fetchAbout,
  });

  const configBackupMutation = useMutation({
    mutationFn: downloadConfigBackup,
    onSuccess: () => {
      messageApi.success("配置备份已开始下载");
    },
    onError: (error: Error) => {
      messageApi.error(`配置备份失败：${error.message}`);
    },
  });

  const stateBackupMutation = useMutation({
    mutationFn: downloadStateBackup,
    onSuccess: () => {
      messageApi.success("状态备份已开始下载");
    },
    onError: (error: Error) => {
      messageApi.error(`状态备份失败：${error.message}`);
    },
  });

  if (aboutQuery.isLoading) {
    return (
      <div className="console-loading">
        <Spin size="large" />
      </div>
    );
  }

  if (aboutQuery.isError || !aboutQuery.data) {
    return (
      <Result
        status="error"
        title="关于页加载失败"
        subTitle={aboutQuery.error instanceof Error ? aboutQuery.error.message : "未知错误"}
      />
    );
  }

  const about = aboutQuery.data;

  return (
    <>
      {contextHolder}
      <Space direction="vertical" size={20} className="console-stack">
        <Row gutter={[18, 18]}>
          <Col xs={24} xl={14}>
            <Card className="surface-card" bordered={false}>
              <div className="section-heading">
                <div>
                  <Typography.Text className="placeholder-kicker">About</Typography.Text>
                  <Typography.Title level={3} className="section-title">
                    运行信息
                  </Typography.Title>
                </div>
                <Space wrap>
                  <Tag color="green">{about.version}</Tag>
                  <Tag>{about.platform}</Tag>
                </Space>
              </div>

              <Descriptions
                column={{ xs: 1, md: 2 }}
                className="provider-detail-descriptions"
                items={[
                  { key: "app", label: "应用名称", children: about.app_name },
                  { key: "version", label: "版本", children: about.version },
                  { key: "go", label: "Go 版本", children: about.go_version },
                  { key: "build", label: "构建时间", children: about.build_time },
                  { key: "config", label: "配置文件", children: about.config_path },
                  { key: "state", label: "状态文件", children: about.state_file },
                  { key: "listen", label: "代理监听", children: about.listen },
                  { key: "admin", label: "管理监听", children: about.admin_listen },
                  { key: "active", label: "当前活跃 Provider", children: about.active_provider },
                  { key: "count", label: "Provider 数量", children: about.provider_count },
                ]}
              />
            </Card>
          </Col>

          <Col xs={24} xl={10}>
            <Space direction="vertical" size={18} className="console-stack">
              <Card className="surface-card" bordered={false}>
                <div className="section-heading">
                  <div>
                    <Typography.Text className="placeholder-kicker">Backup</Typography.Text>
                    <Typography.Title level={4} className="section-title">
                      导出备份
                    </Typography.Title>
                  </div>
                </div>
                <Space wrap>
                  <Button
                    type="primary"
                    loading={configBackupMutation.isPending}
                    onClick={() => configBackupMutation.mutate()}
                  >
                    下载配置备份
                  </Button>
                  <Button
                    loading={stateBackupMutation.isPending}
                    onClick={() => stateBackupMutation.mutate()}
                  >
                    下载状态备份
                  </Button>
                </Space>
              </Card>

              <Card className="surface-card" bordered={false}>
                <Typography.Text className="placeholder-kicker">Capabilities</Typography.Text>
                <Typography.Title level={4} className="section-title">
                  已启用能力
                </Typography.Title>
                <div className="settings-tag-list about-feature-list">
                  {about.features.map((feature) => (
                    <Tag key={feature}>{feature}</Tag>
                  ))}
                </div>
              </Card>
            </Space>
          </Col>
        </Row>
      </Space>
    </>
  );
}
