import { useEffect, useCallback, useRef } from 'react';
import { isWailsEnv } from '../services/apiAdapter';
import { Stock, OrderBook, Telegraph, MarketIndex, MarketStatus } from '../types';

// 事件名称常量，与后端保持一致
const EVENT_STOCK_UPDATE = 'market:stock:update';
const EVENT_ORDERBOOK_UPDATE = 'market:orderbook:update';
const EVENT_TELEGRAPH_UPDATE = 'market:telegraph:update';
const EVENT_MARKET_STATUS_UPDATE = 'market:status:update';
const EVENT_MARKET_INDICES_UPDATE = 'market:indices:update';
const EVENT_MARKET_SUBSCRIBE = 'market:subscribe';
const EVENT_ORDERBOOK_SUBSCRIBE = 'market:orderbook:subscribe';

interface UseMarketEventsOptions {
  onStockUpdate?: (stocks: Stock[]) => void;
  onOrderBookUpdate?: (orderBook: OrderBook) => void;
  onTelegraphUpdate?: (telegraph: Telegraph) => void;
  onMarketStatusUpdate?: (status: MarketStatus) => void;
  onMarketIndicesUpdate?: (indices: MarketIndex[]) => void;
}

// Wails 运行时动态导入
let wailsRuntime: any = null;
const getWailsRuntime = async () => {
  if (!wailsRuntime) {
    wailsRuntime = await import('@wailsjs/runtime/runtime');
  }
  return wailsRuntime;
};

/**
 * 市场数据事件 Hook
 * 监听后端推送的实时市场数据
 * 注意：Web 模式下事件推送不可用，需要轮询实现
 */
export function useMarketEvents(options: UseMarketEventsOptions) {
  const { onStockUpdate, onOrderBookUpdate, onTelegraphUpdate, onMarketStatusUpdate, onMarketIndicesUpdate } = options;

  // 使用 ref 保存回调，避免重复注册
  const stockCallbackRef = useRef(onStockUpdate);
  const orderBookCallbackRef = useRef(onOrderBookUpdate);
  const telegraphCallbackRef = useRef(onTelegraphUpdate);
  const marketStatusCallbackRef = useRef(onMarketStatusUpdate);
  const marketIndicesCallbackRef = useRef(onMarketIndicesUpdate);

  // 更新 ref
  useEffect(() => {
    stockCallbackRef.current = onStockUpdate;
    orderBookCallbackRef.current = onOrderBookUpdate;
    telegraphCallbackRef.current = onTelegraphUpdate;
    marketStatusCallbackRef.current = onMarketStatusUpdate;
    marketIndicesCallbackRef.current = onMarketIndicesUpdate;
  }, [onStockUpdate, onOrderBookUpdate, onTelegraphUpdate, onMarketStatusUpdate, onMarketIndicesUpdate]);

  // 注册事件监听 (仅 Wails 模式)
  useEffect(() => {
    // Web 模式下跳过事件注册
    if (!isWailsEnv()) {
      console.log('Web mode: Wails events not available, real-time updates disabled');
      return;
    }

    let cleanup: (() => void) | undefined;

    (async () => {
      const runtime = await getWailsRuntime();
      
      // 监听股票数据更新
      runtime.EventsOn(EVENT_STOCK_UPDATE, (stocks: Stock[]) => {
        stockCallbackRef.current?.(stocks);
      });

      // 监听盘口数据更新
      runtime.EventsOn(EVENT_ORDERBOOK_UPDATE, (orderBook: OrderBook) => {
        orderBookCallbackRef.current?.(orderBook);
      });

      // 监听快讯数据更新
      runtime.EventsOn(EVENT_TELEGRAPH_UPDATE, (telegraph: Telegraph) => {
        telegraphCallbackRef.current?.(telegraph);
      });

      // 监听市场状态更新
      runtime.EventsOn(EVENT_MARKET_STATUS_UPDATE, (status: MarketStatus) => {
        marketStatusCallbackRef.current?.(status);
      });

      // 监听大盘指数更新
      runtime.EventsOn(EVENT_MARKET_INDICES_UPDATE, (indices: MarketIndex[]) => {
        marketIndicesCallbackRef.current?.(indices);
      });

      cleanup = () => {
        runtime.EventsOff(EVENT_STOCK_UPDATE);
        runtime.EventsOff(EVENT_ORDERBOOK_UPDATE);
        runtime.EventsOff(EVENT_TELEGRAPH_UPDATE);
        runtime.EventsOff(EVENT_MARKET_STATUS_UPDATE);
        runtime.EventsOff(EVENT_MARKET_INDICES_UPDATE);
      };
    })();

    // 清理函数
    return () => {
      cleanup?.();
    };
  }, []);

  // 订阅股票
  const subscribe = useCallback((codes: string[]) => {
    if (!isWailsEnv()) return;
    getWailsRuntime().then(runtime => {
      runtime.EventsEmit(EVENT_MARKET_SUBSCRIBE, codes);
    });
  }, []);

  // 订阅盘口（指定当前选中的股票）
  const subscribeOrderBook = useCallback((code: string) => {
    if (!isWailsEnv()) return;
    getWailsRuntime().then(runtime => {
      runtime.EventsEmit(EVENT_ORDERBOOK_SUBSCRIBE, code);
    });
  }, []);

  return { subscribe, subscribeOrderBook };
}
