import React from 'react';
import { StockChart } from './StockChart';
import { OrderBook } from './OrderBook';
import { Stock, KLineData, OrderBook as OrderBookType, TimePeriod } from '../types';

interface MobileChartPageProps {
  data: KLineData[];
  period: TimePeriod;
  onPeriodChange: (p: TimePeriod) => void;
  stock: Stock;
  orderBook: OrderBookType;
}

export const MobileChartPage: React.FC<MobileChartPageProps> = ({
  data,
  period,
  onPeriodChange,
  stock,
  orderBook
}) => {
  return (
    <div className="flex-1 flex flex-col min-h-0 overflow-hidden">
      {/* 图表区域 - 占60% */}
      <div className="flex-[60] min-h-0 relative">
        <StockChart
          data={data}
          period={period}
          onPeriodChange={onPeriodChange}
          stock={stock}
          isMobile={true}
        />
      </div>

      {/* 盘口区域 - 占40%，始终显示 */}
      <div className="flex-[40] min-h-[180px] border-t fin-divider fin-panel overflow-hidden">
        <OrderBook data={orderBook} isMobile={true} />
      </div>
    </div>
  );
};

export default MobileChartPage;
