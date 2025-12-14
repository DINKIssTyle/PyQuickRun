# 🚀 PyQuickRun & PyQuickBox

**PyQuickRun** and **PyQuickBox** are lightweight tools designed to make running and organizing Python (`.py`) scripts effortless across **Windows, macOS, and Linux**.

---

## 🧩 What is PyQuickRun?

**PyQuickRun** is a launcher that helps you execute Python (`.py`) scripts easily.

- Run scripts by double-clicking
- Choose which Python interpreter to use
- Decide whether to open a terminal or not
- Works even if no special header is defined

---

## 🗂 What is PyQuickBox?

**PyQuickBox** is a launchpad that lets you **collect, categorize, browse, and search** your Python scripts.

- Organize scripts by category
- Grid-based visual browsing
- Search and filter scripts instantly
- Works seamlessly with PyQuickRun

---

## 🔧 Common Feature: `#pqr` Header

Both tools support a special header called `#pqr`, embedded directly in `.py` files.

This header allows you to **predefine**:
- Which Python interpreter to use
- Whether to run in a terminal
- Platform-specific settings (Windows / macOS / Linux)
- Script category (used by PyQuickBox)

---

### Basic `#pqr` Structure
#pqr cat=Category;
win=Path\to\python.exe;
mac=/path/to/python;
linux=/path/to/python;
term=true;

---

## 📌 Examples

### ▶ Run on Windows without opening a terminal
#pqr win=C:\Users\your_user\AppData\Local\Programs\Python\Python310\python.exe;term=false;

### ▶ Run on macOS using a specific .venv
#pqr mac=/Users/your_user/pythons/default/.venv/bin/python;term=false;

### ▶ Category definition for PyQuickBox
#pqr cat=Utilities;win=C:\Users\your_user\AppData\Local\Programs\Python\Python310\python.exe;term=true;


> `cat=` is only used by PyQuickBox for categorization.

---

## ✏ Editing `#pqr`

- You can write `#pqr` directly inside the `.py` file
- Or edit it via the **Properties panel** in PyQuickBox

---

# 📦 Installation Guide

## 🪟 Windows

1. Download the Windows release from  
   https://github.com/DINKIssTyle/PyQuickRun/releases
2. Extract it to any folder
3. Right-click a `.py` file → **Open with** → select **PyQuickRun**
4. Set it as the default app for `.py` files

---

## 🍎 macOS

1. Download the macOS release from  
   https://github.com/DINKIssTyle/PyQuickRun/releases
2. Extract it to `/Applications` or `~/Applications`
3. Select a `.py` file → **Command + I**
4. Set **Open with:** PyQuickRun
5. Click **Change All**

---

## 🐧 Ubuntu / Linux

1. Download the Linux release from  
   https://github.com/DINKIssTyle/PyQuickRun/releases
2. Extract and merge into your `home/` directory
3. Associate `.py` files with PyQuickRun

---

# ▶ PyQuickRun – Basic Usage

- Scripts run even without `#pqr` using **Interpreter Path**
- Click **Browse** to select your default Python binary
- **Run in Terminal / Command**
  - Controls terminal launch
  - Can be overridden by `#pqr term=`
- **Close window after successful execution**
  - Closes PyQuickRun after completion
  - GUI scripts remain active
- **Drag & Drop supported**
- Errors appear in the **status bar**

---

# 🗃 PyQuickBox – Basic Usage

- Register folders containing Python scripts
- Categorization via `#pqr cat=`
- Adjustable grid layout
- Fast search
- Theme support: Dark / Light / System
- Settings:
  - Default interpreter
  - UI scale
  - Script name font size
- Remove folders with the trash icon

### 💡 Tips

- Drag folders into the main window to register them
- Drag `.py` files to run instantly (PyQuickRun behavior)
- If an `icon` folder exists inside a script directory, PyQuickBox will use it for custom icons.
- Place a `.png` file with the **same name as the Python script** to assign a custom icon.
  
  Example:
  my_tool.py
  icon/my_tool.png

- You can also define a fallback icon by adding:
  icon/default.png

  This image will be used for scripts that do not have a matching icon file.

---

## ✨ Why PyQuickRun & PyQuickBox?

- No complex setup
- No project lock-in
- Supports system Python, `.venv`, and custom interpreters
- Ideal for utilities, tools, and script collections

