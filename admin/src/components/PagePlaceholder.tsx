import { Empty, Typography } from 'antd'

interface PagePlaceholderProps {
  title: string
  description?: string
}

// Stand-in for pages that don't have real content yet. Swap this out once
// the page's actual UI is built — nothing else references it.
export default function PagePlaceholder({ title, description }: PagePlaceholderProps) {
  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        {title}
      </Typography.Title>
      <Empty description={description ?? '页面开发中'} style={{ marginTop: 80 }} />
    </div>
  )
}
