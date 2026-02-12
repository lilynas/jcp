import React from 'react';
import { OrderBook as OrderBookType } from '../types';

interface OrderBookProps {
  data: OrderBookType;
  isMobile?: boolean;
}

export const OrderBook: React.FC<OrderBookProps> = ({ data, isMobile = false }) => {
  // 安全检查：确保 data 及其属性存在
  const bids = data?.bids ?? [];
  const asks = data?.asks ?? [];

  // 计算委比：(委买量 - 委卖量) / (委买量 + 委卖量) * 100%
  const totalBidSize = bids.reduce((sum, b) => sum + b.size, 0);
  const totalAskSize = asks.reduce((sum, a) => sum + a.size, 0);
  const totalSize = totalBidSize + totalAskSize;

  const weibi = totalSize > 0
    ? ((totalBidSize - totalAskSize) / totalSize * 100).toFixed(2)
    : '0.00';
  const weibiBuy = totalSize > 0 ? (totalBidSize / totalSize * 100).toFixed(1) : '0';
  const weibiSell = totalSize > 0 ? (totalAskSize / totalSize * 100).toFixed(1) : '0';

  // 移动端简化显示
  if (isMobile) {
    return (
      <div className="h-full flex flex-col fin-panel overflow-hidden text-xs font-mono select-none">
        {/* 委比信息 - 移动端顶部显示 */}
        <div className="flex items-center justify-between px-3 py-1.5 border-b fin-divider fin-panel-strong">
          <div className="flex items-center gap-2">
            <span className="text-slate-500 text-[10px]">委比</span>
            <span className={`font-bold text-sm ${parseFloat(weibi) >= 0 ? 'text-red-400' : 'text-green-400'}`}>
              {weibi}%
            </span>
          </div>
          <div className="text-[10px] text-slate-500 flex items-center gap-1">
            <span className="text-red-400">买 {weibiBuy}%</span>
            <span className="text-slate-600">|</span>
            <span className="text-green-400">卖 {weibiSell}%</span>
          </div>
        </div>

        {/* 买卖盘 - 移动端横向排列 */}
        <div className="flex-1 flex min-h-0">
          {/* 买盘 */}
          <div className="flex-1 flex flex-col border-r fin-divider">
            <div className="p-1.5 border-b fin-divider font-bold text-[10px] text-slate-400 flex justify-between fin-panel-strong">
              <span>买盘</span>
              <span className="font-normal opacity-70">量</span>
            </div>
            <div className="flex-1 overflow-hidden">
              {bids.slice(0, 5).map((bid, i) => (
                <div key={`bid-${i}`} className="relative flex justify-between px-2 py-0.5">
                  <div 
                    className="absolute top-0 left-0 bottom-0 bg-green-900/20" 
                    style={{ width: `${Math.min(bid.percent * 3, 100)}%` }}
                  />
                  <span className="text-green-400 relative z-10">{bid.price.toFixed(2)}</span>
                  <span className="text-slate-300 relative z-10 text-[10px]">{bid.size}</span>
                </div>
              ))}
            </div>
          </div>

          {/* 卖盘 */}
          <div className="flex-1 flex flex-col">
            <div className="p-1.5 border-b fin-divider font-bold text-[10px] text-slate-400 flex justify-between fin-panel-strong">
              <span>卖盘</span>
              <span className="font-normal opacity-70">量</span>
            </div>
            <div className="flex-1 overflow-hidden">
              {asks.slice(0, 5).reverse().map((ask, i) => (
                <div key={`ask-${i}`} className="relative flex justify-between px-2 py-0.5">
                  <div 
                    className="absolute top-0 right-0 bottom-0 bg-red-900/20" 
                    style={{ width: `${Math.min(ask.percent * 3, 100)}%` }} 
                  />
                  <span className="text-red-400 relative z-10">{ask.price.toFixed(2)}</span>
                  <span className="text-slate-300 relative z-10 text-[10px]">{ask.size}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    );
  }

  // 桌面端原始显示
  return (
    <div className="h-full flex flex-row fin-panel border-l fin-divider overflow-hidden text-xs font-mono select-none">
       {/* 买盘 */}
       <div className="flex-1 flex flex-col border-r fin-divider">
          <div className="p-2 border-b fin-divider font-bold text-slate-400 flex justify-between fin-panel-strong">
             <span>买盘</span>
             <span className="text-[10px] font-normal opacity-70">数量</span>
          </div>
          <div className="flex-1 overflow-hidden">
             {bids.slice(0, 15).map((bid, i) => (
                <div key={`bid-${i}`} className="relative flex justify-between px-2 py-0.5 hover:bg-slate-800/50 cursor-crosshair">
                   <div 
                    className="absolute top-0 left-0 bottom-0 bg-green-900/20 transition-all duration-300" 
                    style={{ width: `${Math.min(bid.percent * 5, 100)}%` }}
                  />
                  <span className="text-green-400 relative z-10">{bid.price.toFixed(2)}</span>
                  <span className="text-slate-300 relative z-10">{bid.size}</span>
                </div>
             ))}
          </div>
       </div>

       {/* 委比信息 */}
       <div className="w-24 flex flex-col items-center justify-center border-r fin-divider fin-panel-strong z-10 shadow-inner">
           <div className="text-slate-500 text-[10px]">委比</div>
           <div className={`font-bold my-1 ${parseFloat(weibi) >= 0 ? 'text-red-400' : 'text-green-400'}`}>{weibi}%</div>
           <div className="text-[10px] text-slate-500">
             <span className="text-red-400">{weibiBuy}%</span>
             <span className="mx-1">/</span>
             <span className="text-green-400">{weibiSell}%</span>
           </div>
       </div>

       {/* 卖盘 */}
       <div className="flex-1 flex flex-col">
          <div className="p-2 border-b fin-divider font-bold text-slate-400 flex justify-between fin-panel-strong">
             <span>卖盘</span>
             <span className="text-[10px] font-normal opacity-70">数量</span>
          </div>
          <div className="flex-1 overflow-hidden">
            {asks.slice(0, 15).map((ask, i) => (
                <div key={`ask-${i}`} className="relative flex justify-between px-2 py-0.5 hover:bg-slate-800/50 cursor-crosshair">
                   <div 
                    className="absolute top-0 right-0 bottom-0 bg-red-900/20 transition-all duration-300" 
                    style={{ width: `${Math.min(ask.percent * 5, 100)}%` }} 
                  />
                  <span className="text-red-400 relative z-10">{ask.price.toFixed(2)}</span>
                  <span className="text-slate-300 relative z-10">{ask.size}</span>
                </div>
            ))}
          </div>
       </div>
    </div>
  );
};
