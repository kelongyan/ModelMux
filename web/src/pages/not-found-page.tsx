import { Button, Result, Space } from "antd";
import { useNavigate } from "react-router-dom";

export function NotFoundPage(): JSX.Element {
  const navigate = useNavigate();

  return (
    <div className="console-loading">
      <Result
        status="404"
        title="页面不存在"
        subTitle="您访问的页面不存在或已被移除。"
        extra={
          <Space>
            <Button type="primary" onClick={() => navigate("/dashboard")}>
              返回首页
            </Button>
            <Button onClick={() => navigate(-1)}>返回上一页</Button>
          </Space>
        }
      />
    </div>
  );
}
