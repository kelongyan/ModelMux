import { Card, Typography } from "antd";

type PlaceholderPageProps = {
  title: string;
  description: string;
};

// PlaceholderPage 为尚未完成的页面提供统一占位布局，方便阶段化交付。
export function PlaceholderPage({ title, description }: PlaceholderPageProps): JSX.Element {
  return (
    <Card className="surface-card placeholder-card" bordered={false}>
      <Typography.Text className="placeholder-kicker">阶段占位</Typography.Text>
      <Typography.Title level={3}>{title}</Typography.Title>
      <Typography.Paragraph>{description}</Typography.Paragraph>
    </Card>
  );
}
