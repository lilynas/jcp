import { isWailsEnv, httpRequest } from './apiAdapter';

export interface UpdateInfo {
  hasUpdate: boolean;
  latestVersion: string;
  currentVersion: string;
  releaseUrl: string;
  releaseNotes: string;
  error?: string;
}

export interface UpdateProgress {
  status: 'checking' | 'downloading' | 'installing' | 'completed' | 'error';
  message: string;
  percent: number;
}

// 动态加载 Wails API
let wailsApp: any = null;
const getWailsApp = async () => {
  if (!wailsApp && isWailsEnv()) {
    wailsApp = await import('../../wailsjs/go/main/App');
  }
  return wailsApp;
};

// 动态加载 Wails 运行时
let wailsRuntime: any = null;
const getWailsRuntime = async () => {
  if (!wailsRuntime && isWailsEnv()) {
    wailsRuntime = await import('../../wailsjs/runtime/runtime');
  }
  return wailsRuntime;
};

export async function checkForUpdate(): Promise<UpdateInfo> {
  if (isWailsEnv()) {
    const app = await getWailsApp();
    return app.CheckForUpdate();
  } else {
    // Web 模式不支持更新
    return {
      hasUpdate: false,
      latestVersion: '',
      currentVersion: '',
      releaseUrl: '',
      releaseNotes: '',
    };
  }
}

export async function doUpdate(): Promise<string> {
  if (isWailsEnv()) {
    const app = await getWailsApp();
    return app.DoUpdate();
  } else {
    return 'Updates not supported in web mode';
  }
}

export async function restartApp(): Promise<string> {
  if (isWailsEnv()) {
    const app = await getWailsApp();
    return app.RestartApp();
  } else {
    // Web 模式下刷新页面
    window.location.reload();
    return 'ok';
  }
}

export async function getCurrentVersion(): Promise<string> {
  if (isWailsEnv()) {
    const app = await getWailsApp();
    return app.GetCurrentVersion();
  } else {
    const response = await httpRequest<{version: string}>('/api/version');
    return response.version;
  }
}

export function onUpdateProgress(callback: (progress: UpdateProgress) => void): () => void {
  if (!isWailsEnv()) {
    // Web 模式不支持更新进度
    return () => {};
  }

  let cleanup: (() => void) | undefined;
  
  getWailsRuntime().then(runtime => {
    if (runtime) {
      runtime.EventsOn('update:progress', callback);
      cleanup = () => runtime.EventsOff('update:progress');
    }
  });

  return () => {
    if (cleanup) cleanup();
  };
}
