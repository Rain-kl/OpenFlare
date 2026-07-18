import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';

import { RankChart } from './rank-chart';

type RankCardItem = {
  label: string;
  value: number;
};

type RankCardProps = {
  title: string;
  description: string;
  items: RankCardItem[];
  color?: string;
};

export function RankCard({
  title,
  description,
  items,
  color = '#3b82f6',
}: RankCardProps) {
  return (
    <Card className='border-dashed shadow-none'>
      <CardHeader className='pb-3'>
        <CardTitle className='text-sm font-semibold text-foreground'>
          {title}
        </CardTitle>
        <CardDescription className='text-xs text-muted-foreground'>
          {description}
        </CardDescription>
      </CardHeader>
      <CardContent className='pt-0'>
        <RankChart
          items={items}
          color={color}
          emptyMessage={`暂无 ${title} 数据`}
        />
      </CardContent>
    </Card>
  );
}
