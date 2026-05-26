# Win7 — OpenGL 2.1 troubleshooting

**🌐 Languages**: [English](#english) | [Русский](#русский)

---

## English

### Symptom

On Windows 7 (32-bit, legacy build `singbox-launcher-<version>-win7-32.zip`):

- launcher tray icon appears
- main window does not open, or opens as an empty frame (no widgets, no text)
- other Fyne-free apps (Throne, Happ, regular Win32 apps) work fine on the same machine

### Cause

The launcher UI is built on Fyne, which uses GLFW + OpenGL for rendering. **GLFW requires OpenGL 2.1 or newer.** On older hardware with integrated graphics (Intel HD Graphics 1xxx-2xxx, some ATI/AMD chipsets), Win7 ships with OpenGL 2.0 only — the GPU driver doesn't expose 2.1.

You can confirm the OpenGL version with [GPU-Z](https://www.techpowerup.com/gpuz/), [GLview](https://www.realtech-vr.com/home/glview), or any tool that reads `glGetString(GL_VERSION)`.

### Fix — drop in Mesa3D software OpenGL

[Mesa3D](https://www.mesa3d.org/) provides a software OpenGL implementation. The pre-built `mesa-dist-win` package ships ready-to-use DLLs that override the system OpenGL with a 3.x-capable software renderer, transparent to the application.

**Steps:**

1. Download the latest **32-bit (x86)** release of `mesa-dist-win`:

   <https://github.com/pal1000/mesa-dist-win/releases>

   Pick `mesa3d-<version>-release-msvc.7z` (or `.exe` self-extracting variant).

2. Extract the archive. Inside, navigate to the `x86/` subfolder.

3. Copy **`opengl32.dll`** and **`libglapi.dll`** from `x86/` into the same folder where `singbox-launcher-win7-32.exe` lives.

4. Restart the launcher.

The Windows DLL search order will pick up the local `opengl32.dll` before the system one, and Fyne / GLFW will see an OpenGL 3.x context — the window will render normally.

### Verification

- Tray icon → click → main window opens with Core / Servers / Settings tabs.
- If it still doesn't render, double-check that both DLLs were copied from the `x86/` subfolder (not `x64/`) and sit next to the `.exe`, not in a subdirectory.

### Performance note

Mesa software rendering is slower than a hardware GPU driver. For a small launcher UI like this it's still smooth (<10 ms per frame on a typical Win7 machine), but expect higher CPU usage than the Windows 10/11 build with native OpenGL.

### Why isn't this bundled?

Mesa3D ships under a permissive license (MIT-style), so technically we could ship the DLLs in the Win7 release archive. We don't, because:

- It would inflate the Win7 release by ~5 MB just for legacy hardware.
- Most Win7 32-bit users have OpenGL 2.1+ available and don't need it.
- The launcher would have to detect missing OpenGL 2.1 at startup and lazy-extract — extra complexity for an edge case.

If demand grows, we may revisit and either bundle or auto-download Mesa on first failed startup.

---

## Русский

### Симптом

На Windows 7 (32-bit, legacy-сборка `singbox-launcher-<version>-win7-32.zip`):

- иконка в трее появляется
- главное окно не открывается, либо открывается пустая рамка (без виджетов, без текста)
- другие приложения без Fyne (Throne, Happ, обычные Win32-программы) на той же машине работают нормально

### Причина

UI лаунчера построен на Fyne, который использует GLFW + OpenGL для рендера. **GLFW требует OpenGL 2.1 или новее.** На старом железе с интегрированной графикой (Intel HD Graphics 1xxx-2xxx, некоторые чипсеты ATI/AMD) Win7 поставляется только с OpenGL 2.0 — GPU-драйвер не отдаёт 2.1.

Проверить версию OpenGL: [GPU-Z](https://www.techpowerup.com/gpuz/), [GLview](https://www.realtech-vr.com/home/glview) или любой инструмент, который читает `glGetString(GL_VERSION)`.

### Решение — положить Mesa3D software OpenGL

[Mesa3D](https://www.mesa3d.org/) — software-реализация OpenGL. Пакет `mesa-dist-win` поставляет готовые DLL, которые подменяют системный OpenGL software-рендерером с поддержкой 3.x, прозрачно для приложения.

**Шаги:**

1. Скачать последний **32-bit (x86)** релиз `mesa-dist-win`:

   <https://github.com/pal1000/mesa-dist-win/releases>

   Берите `mesa3d-<version>-release-msvc.7z` (или `.exe` самораспаковывающийся вариант).

2. Распаковать архив. Внутри зайти в подпапку `x86/`.

3. Скопировать **`opengl32.dll`** и **`libglapi.dll`** из `x86/` в ту же папку, где лежит `singbox-launcher-win7-32.exe`.

4. Перезапустить лаунчер.

Порядок поиска DLL в Windows подхватит локальный `opengl32.dll` раньше системного, и Fyne / GLFW увидят OpenGL 3.x контекст — окно отрендерится нормально.

### Проверка

- Иконка в трее → клик → главное окно открывается со вкладками Core / Servers / Settings.
- Если по-прежнему не рендерит — проверьте что обе DLL скопированы из подпапки `x86/` (не `x64/`), и лежат рядом с `.exe`, не в подкаталоге.

### Замечание про производительность

Software-рендеринг Mesa медленнее, чем hardware GPU-драйвер. Для маленького UI лаунчера это всё равно гладко (<10 ms на кадр на типичной Win7-машине), но CPU-usage будет выше, чем на Windows 10/11 сборке с native OpenGL.

### Почему не bundled?

Mesa3D под permissive-лицензией (MIT-style), технически могли бы поставлять DLL в Win7-архиве. Не делаем, потому что:

- Это раздуло бы Win7-релиз на ~5 MB ради legacy-железа.
- У большинства Win7 32-bit пользователей OpenGL 2.1+ доступен, им это не нужно.
- Лаунчер должен был бы детектить missing OpenGL 2.1 на старте и lazy-extract — лишняя сложность ради edge case'а.

Если запросов станет много — рассмотрим bundle / автозагрузку Mesa при первом неудачном старте.

### Кредит

Кейс и решение прислал пользователь из Telegram-канала, спасибо за репорт.
