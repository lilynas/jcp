import { useEffect, useCallback, useRef } from 'react';
import { isWailsEnv, httpRequest } from '../services/apiAdapter';
import { Stock, OrderBook, Telegraph, MarketIndex, MarketStatus, KLineData } from '../types';
import { getStockRealTimeData, getOrderBook } from '../services/stockService';

// K线推送数据结构
interface KLineUpdateData {
  code: string;
  period: string;
  data: KLineData[];
}

// 事件名称常量，与后端保持一致
const EVENT_STOCK_UPDATE = 'market:stock:update';
const EVENT_ORDERBOOK_UPDATE = 'market:orderbook:update';
const EVENT_TELEGRAPH_UPDATE = 'market:telegraph:update';
const EVENT_MARKET_STATUS_UPDATE = 'market:status:update';
const EVENT_MARKET_INDICES_UPDATE = 'market:indices:update';
const EVENT_MARKET_SUBSCRIBE = 'market:subscribe';
const EVENT_ORDERBOOK_SUBSCRIBE = 'market:orderbook:subscribe';
const EVENT_KLINE_UPDATE = 'market:kline:update';
const EVENT_KLINE_SUBSCRIBE = 'market:kline:subscribe';

interface UseMarketEventsOptions {
  onStockUpdate?: (stocks: Stock[]) => void;
  onOrderBookUpdate?: (orderBook: OrderBook) => void;
  onTelegraphUpdate?: (telegraph: Telegraph) => void;
  onMarketStatusUpdate?: (status: MarketStatus) => void;
  onMarketIndicesUpdate?: (indices: MarketIndex[]) => void;
  onKLineUpdate?: (data: KLineUpdateData) => void;
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
 * Wails 模式: 监听后端推送的实时市场数据
 * Web 模式: 轮询 HTTP API 获取数据
 */
export function useMarketEvents(options: UseMarketEventsOptions) {
  const { onStockUpdate, onOrderBookUpdate, onTelegraphUpdate, onMarketStatusUpdate, onMarketIndicesUpdate, onKLineUpdate } = options;

  // 使用 ref 保存回调，避免重复注册
  const stockCallbackRef = useRef(onStockUpdate);
  const orderBookCallbackRef = useRef(onOrderBookUpdate);
  const telegraphCallbackRef = useRef(onTelegraphUpdate);
  const marketStatusCallbackRef = useRef(onMarketStatusUpdate);
  const marketIndicesCallbackRef = useRef(onMarketIndicesUpdate);
  const klineCallbackRef = useRef(onKLineUpdate);

  // Web 模式下的订阅状态
  const webSubscribedCodesRef = useRef<string[]>([]);
  const webOrderBookCodeRef = useRef<string>('');
  const lastTelegraphContentRef = useRef<string>('');

  // 更新 ref
  useEffect(() => {
    stockCallbackRef.current = onStockUpdate;
    orderBookCallbackRef.current = onOrderBookUpdate;
    telegraphCallbackRef.current = onTelegraphUpdate;
    marketStatusCallbackRef.current = onMarketStatusUpdate;
    marketIndicesCallbackRef.current = onMarketIndicesUpdate;
    klineCallbackRef.current = onKLineUpdate;
  }, [onStockUpdate, onOrderBookUpdate, onTelegraphUpdate, onMarketStatusUpdate, onMarketIndicesUpdate, onKLineUpdate]);

  // ========== Wails 模式: 事件监听 ==========
  useEffect(() => {
    if (!isWailsEnv()) return;

    let cleanup: (() => void) | undefined;

    (async () => {
      const runtime = await getWailsRuntime();
      
      runtime.EventsOn(EVENT_STOCK_UPDATE, (stocks: Stock[]) => {
        stockCallbackRef.current?.(stocks);
      });

      runtime.EventsOn(EVENT_ORDERBOOK_UPDATE, (orderBook: OrderBook) => {
        orderBookCallbackRef.current?.(orderBook);
      });

      runtime.EventsOn(EVENT_TELEGRAPH_UPDATE, (telegraph: Telegraph) => {
        telegraphCallbackRef.current?.(telegraph);
      });

      runtime.EventsOn(EVENT_MARKET_STATUS_UPDATE, (status: MarketStatus) => {
        marketStatusCallbackRef.current?.(status);
      });

      runtime.EventsOn(EVENT_MARKET_INDICES_UPDATE, (indices: MarketIndex[]) => {
        marketIndicesCallbackRef.current?.(indices);
      });

      // 监听K线数据更新
      runtime.EventsOn(EVENT_KLINE_UPDATE, (data: KLineUpdateData) => {
        klineCallbackRef.current?.(data);
      });

      cleanup = () => {
        runtime.EventsOff(EVENT_STOCK_UPDATE);
        runtime.EventsOff(EVENT_ORDERBOOK_UPDATE);
        runtime.EventsOff(EVENT_TELEGRAPH_UPDATE);
        runtime.EventsOff(EVENT_MARKET_STATUS_UPDATE);
        runtime.EventsOff(EVENT_MARKET_INDICES_UPDATE);
        runtime.EventsOff(EVENT_KLINE_UPDATE);
      };
    })();

    return () => {
      cleanup?.();
    };
  }, []);

  // ========== Web 模式: 轮询 ==========
  useEffect(() => {
    if (isWailsEnv()) return;

    console.log('Web mode: Starting polling for market data');

    // 股票数据轮询 (3s)
    const stockTimer = setInterval(async () => {
      const codes = webSubscribedCodesRef.current;
      if (codes.length === 0) return;
      try {
        const stocks = await getStockRealTimeData(codes);
        stockCallbackRef.current?.(stocks);
      } catch (e) {
        // 静默失败，下次重试
      }
    }, 3000);

    // 盘口数据轮询 (1s)
    const orderBookTimer = setInterval(async () => {
      const code = webOrderBookCodeRef.current;
      if (!code) return;
      try {
        const ob = await getOrderBook(code);
        orderBookCallbackRef.current?.(ob);
      } catch (e) {
        // 静默失败
      }
    }, 1000);

    // 快讯轮询 (30s)
    const telegraphTimer = setInterval(async () => {
      try {
        const telegraphs = await httpRequest<Telegraph[]>('/api/news/telegraph');
        if (telegraphs && telegraphs.length > 0) {
          const latest = telegraphs[0];
          if (latest.content !== lastTelegraphContentRef.current) {
            lastTelegraphContentRef.current = latest.content;
            telegraphCallbackRef.current?.(latest);
          }
        }
      } catch (e) {
        // 静默失败
      }
    }, 30000);

    // 市场状态轮询 (5s)
    const marketStatusTimer = setInterval(async () => {
      try {
        const status = await httpRequest<MarketStatus>('/api/market/status');
        marketStatusCallbackRef.current?.(status);
      } catch (e) {
        // 静默失败
      }
    }, 5000);

    // 大盘指数轮询 (3s)
    const marketIndicesTimer = setInterval(async () => {
      try {
        const indices = await httpRequest<MarketIndex[]>('/api/market/indices');
        marketIndicesCallbackRef.current?.(indices);
      } catch (e) {
        // 静默失败
      }
    }, 3000);

    // 立即执行一次
    (async () => {
      try {
        const status = await httpRequest<MarketStatus>('/api/market/status');
        marketStatusCallbackRef.current?.(status);
      } catch (e) { /* ignore */ }

      try {
        const indices = await httpRequest<MarketIndex[]>('/api/market/indices');
        marketIndicesCallbackRef.current?.(indices);
      } catch (e) { /* ignore */ }

      try {
        const telegraphs = await httpRequest<Telegraph[]>('/api/news/telegraph');
        if (telegraphs && telegraphs.length > 0) {
          const latest = telegraphs[0];
          lastTelegraphContentRef.current = latest.content;
          telegraphCallbackRef.current?.(latest);
        }
      } catch (e) { /* ignore */ }

      // 初始股票数据
      const codes = webSubscribedCodesRef.current;
      if (codes.length > 0) {
        try {
          const stocks = await getStockRealTimeData(codes);
          stockCallbackRef.current?.(stocks);
        } catch (e) { /* ignore */ }
      }
    })();

    return () => {
      clearInterval(stockTimer);
      clearInterval(orderBookTimer);
      clearInterval(telegraphTimer);
      clearInterval(marketStatusTimer);
      clearInterval(marketIndicesTimer);
    };
  }, []);

  // 订阅股票 (Web 模式下立即拉取一次)
  const subscribe = useCallback((codes: string[]) => {
    if (isWailsEnv()) {
      getWailsRuntime().then(runtime => {
        runtime.EventsEmit(EVENT_MARKET_SUBSCRIBE, codes);
      });
    } else {
      webSubscribedCodesRef.current = codes;
      // 立即拉取一次数据
      if (codes.length > 0) {
        getStockRealTimeData(codes).then(stocks => {
          stockCallbackRef.current?.(stocks);
        }).catch(() => {});
      }
    }
  }, []);

  // 订阅盘口（指定当前选中的股票）
  const subscribeOrderBook = useCallback((code: string) => {
    if (isWailsEnv()) {
      getWailsRuntime().then(runtime => {
        runtime.EventsEmit(EVENT_ORDERBOOK_SUBSCRIBE, code);
      });
    } else {
      webOrderBookCodeRef.current = code;
    }
  }, []);

  // 订阅K线（指定股票代码和周期）
  const subscribeKLine = useCallback((code: string, period: string) => {
    if (isWailsEnv()) {
      getWailsRuntime().then(runtime => {
        runtime.EventsEmit(EVENT_KLINE_SUBSCRIBE, code, period);
      });
    }
    // Web 模式暂不支持 K线推送，使用轮询方式
  }, []);

  return { subscribe, subscribeOrderBook, subscribeKLine };
}
