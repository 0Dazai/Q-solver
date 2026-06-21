import {
  CancelRunningTask,
  CheckScreenCapturePermission,
  ClearResume,
  GetDomainCategories,
  GetInitStatus,
  GetModels,
  GetScreenshotPreview,
  GetSettings,
  MoveWindow,
  OpenScreenCaptureSettings,
  ParseResume,
  RemoveFocus,
  RequestScreenCapturePermission,
  RestoreFocus,
  SaveImageToFile,
  ScrollContent,
  SelectResume,
  SetWindowAlwaysOnTop,
  StartRecordingKey,
  StopRecordingKey,
  TestConnection,
  ToggleClickThrough,
  ToggleMinimizeWindow,
  ToggleVisibility,
  TriggerSolve,
  TriggerScreenshot,
  TriggerSend,
  RemoveScreenshot,
  ClearScreenshots,
  UpdateSettings,
} from '../../wailsjs/go/app/App'

import { Quit } from '../../wailsjs/runtime/runtime'

export const api = {
  getSettings: () => GetSettings(),
  syncSettings: (json) => UpdateSettings(json),
  updateSettings: (json) => UpdateSettings(json),

  getModels: (apiKey, baseURL, provider) => GetModels(apiKey, baseURL, provider),
  testConnection: (apiKey, baseURL, provider, model) => TestConnection(apiKey, baseURL, provider, model),

  triggerSolve: () => TriggerSolve(),
  triggerScreenshot: () => TriggerScreenshot(),
  triggerSend: () => TriggerSend(),
  removeScreenshot: (index) => RemoveScreenshot(index),
  clearScreenshots: () => ClearScreenshots(),
  cancelTask: () => CancelRunningTask(),

  startRecordingKey: (action) => StartRecordingKey(action),
  stopRecordingKey: () => StopRecordingKey(),

  selectResume: () => SelectResume(),
  clearResume: () => ClearResume(),
  parseResume: () => ParseResume(),

  restoreFocus: () => RestoreFocus(),
  removeFocus: () => RemoveFocus(),
  moveWindow: (x, y) => MoveWindow(x, y),
  scrollContent: (dir) => ScrollContent(dir),
  setAlwaysOnTop: (v) => SetWindowAlwaysOnTop(v),
  toggleVisibility: () => ToggleVisibility(),
  toggleClickThrough: () => ToggleClickThrough(),
  toggleMinimizeWindow: () => ToggleMinimizeWindow(),
  quit: () => Quit(),

  getInitStatus: () => GetInitStatus(),
  getDomainCategories: () => GetDomainCategories(),

  getScreenshotPreview: (q, s, g, n, m) => GetScreenshotPreview(q, s, g, n, m),
  checkScreenCapturePermission: () => CheckScreenCapturePermission(),
  requestScreenCapturePermission: () => RequestScreenCapturePermission(),
  openScreenCaptureSettings: () => OpenScreenCaptureSettings(),

  saveImageToFile: (b64) => SaveImageToFile(b64),
}
