import { useState, useEffect, useCallback } from 'react';

export type DeviceType = 'desktop' | 'tablet' | 'mobile';
export type MobileTab = 'watchlist' | 'chart' | 'chat';

interface ResponsiveState {
  deviceType: DeviceType;
  isMobile: boolean;
  isTablet: boolean;
  isDesktop: boolean;
  activeTab: MobileTab;
  setActiveTab: (tab: MobileTab) => void;
}

const getDeviceType = (width: number): DeviceType => {
  if (width >= 1024) return 'desktop';
  if (width >= 768) return 'tablet';
  return 'mobile';
};

export const useResponsive = (): ResponsiveState => {
  const [deviceType, setDeviceType] = useState<DeviceType>('desktop');
  const [activeTab, setActiveTabState] = useState<MobileTab>('chart');

  useEffect(() => {
    const checkDevice = () => {
      const width = window.innerWidth;
      setDeviceType(getDeviceType(width));
    };

    checkDevice();
    window.addEventListener('resize', checkDevice);
    return () => window.removeEventListener('resize', checkDevice);
  }, []);

  const setActiveTab = useCallback((tab: MobileTab) => {
    setActiveTabState(tab);
  }, []);

  return {
    deviceType,
    isMobile: deviceType === 'mobile',
    isTablet: deviceType === 'tablet',
    isDesktop: deviceType === 'desktop',
    activeTab,
    setActiveTab,
  };
};

export default useResponsive;
