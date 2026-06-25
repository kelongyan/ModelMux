import { Modal, Table, Typography } from "antd";

type ShortcutsHelpProps = {
  open: boolean;
  onClose: () => void;
};

type ShortcutItem = {
  key: string;
  description: string;
};

const shortcuts: ShortcutItem[] = [
  { key: "Ctrl/Cmd + R", description: "重载配置" },
  { key: "g → d", description: "跳转到 Dashboard" },
  { key: "g → p", description: "跳转到 Providers" },
  { key: "g → t", description: "跳转到 Stats (统计)" },
  { key: "g → s", description: "跳转到 Settings (设置)" },
  { key: "g → e", description: "跳转到 Events (事件)" },
  { key: "g → a", description: "跳转到 About (关于)" },
  { key: "?", description: "显示此帮助面板" },
];

const columns = [
  {
    title: "快捷键",
    dataIndex: "key",
    key: "key",
    width: 160,
    render: (key: string) => (
      <Typography.Text keyboard style={{ fontFamily: "monospace" }}>
        {key}
      </Typography.Text>
    ),
  },
  {
    title: "功能",
    dataIndex: "description",
    key: "description",
  },
];

export function ShortcutsHelp({ open, onClose }: ShortcutsHelpProps): JSX.Element {
  return (
    <Modal
      title="键盘快捷键"
      open={open}
      onCancel={onClose}
      footer={null}
      width={480}
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        使用以下快捷键可以快速导航和操作：
      </Typography.Paragraph>
      <Table
        dataSource={shortcuts}
        columns={columns}
        rowKey="key"
        size="small"
        pagination={false}
        showHeader={false}
      />
      <Typography.Paragraph type="secondary" style={{ marginTop: 16, fontSize: "0.85em" }}>
        提示：g → d 表示先按 g，等待 1.5 秒内再按 d。
      </Typography.Paragraph>
    </Modal>
  );
}
