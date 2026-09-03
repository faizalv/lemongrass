import { app, shell, BrowserWindow } from 'electron'
import { join } from 'path'
import { electronApp, optimizer, is } from '@electron-toolkit/utils'
import icon from '../../resources/icon.png?asset'
import { registerPtyHandlers, killAllShells } from './pty'
import { registerProjectHandlers } from './projects'
import { registerKnowledgeHandlers } from './knowledge'
import { registerLayoutHandlers } from './layouts'
import { registerWindowControlHandlers, wireMaximizeEvents } from './windowControls'

let mainWindow: BrowserWindow | undefined

function createWindow(): void {
  const win = new BrowserWindow({
    width: 900,
    height: 670,
    show: false,
    frame: false,
    autoHideMenuBar: true,
    ...(process.platform === 'linux' ? { icon } : {}),
    webPreferences: {
      preload: join(__dirname, '../preload/index.js'),
      sandbox: false
    }
  })
  mainWindow = win
  wireMaximizeEvents(win)

  win.on('ready-to-show', () => {
    win.show()
  })

  // Otherwise mainWindow keeps pointing at a destroyed BrowserWindow after
  // close -- accessing .webContents on it throws "Object has been
  // destroyed" for any handler still holding this getter.
  win.on('closed', () => {
    mainWindow = undefined
  })

  win.webContents.setWindowOpenHandler((details) => {
    shell.openExternal(details.url)
    return { action: 'deny' }
  })

  if (is.dev && process.env['ELECTRON_RENDERER_URL']) {
    win.loadURL(process.env['ELECTRON_RENDERER_URL'])
  } else {
    win.loadFile(join(__dirname, '../renderer/index.html'))
  }
}

app.whenReady().then(() => {
  electronApp.setAppUserModelId('com.lemongrass.app')

  // Toggles DevTools with F12 in dev, blocks CommandOrControl+R in prod.
  app.on('browser-window-created', (_, window) => {
    optimizer.watchWindowShortcuts(window)
  })

  registerPtyHandlers(() => mainWindow?.webContents)
  registerProjectHandlers(() => mainWindow)
  registerKnowledgeHandlers()
  registerLayoutHandlers()
  registerWindowControlHandlers(() => mainWindow)

  createWindow()

  app.on('activate', function () {
    // On macOS it's common to re-create a window in the app when the
    // dock icon is clicked and there are no other windows open.
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })
})

// macOS apps conventionally stay alive with no windows open until Cmd+Q.
app.on('window-all-closed', () => {
  killAllShells()
  if (process.platform !== 'darwin') {
    app.quit()
  }
})
