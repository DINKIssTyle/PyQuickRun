#nullable disable

using System;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using Microsoft.Win32;

namespace PyQuickRun
{
    public partial class MainWindow : Window
    {
        public MainWindow()
        {
            InitializeComponent();
            LoadSettings();

            // [추가된 부분 1] 앱이 화면에 다 그려진 뒤에 "파일을 들고 왔는지" 검사
            this.Loaded += MainWindow_Loaded;
        }

        // [추가된 부분 2] 실행 시 전달된 파일(더블클릭/Open With) 처리
        private void MainWindow_Loaded(object sender, RoutedEventArgs e)
        {
            string[] args = Environment.GetCommandLineArgs();

            // args[0]은 실행파일 자체의 경로이고, args[1]부터가 실제 전달된 파일입니다.
            if (args.Length > 1)
            {
                string filePath = args[1];
                if (File.Exists(filePath) && Path.GetExtension(filePath).ToLower() == ".py")
                {
                    // 파일을 받았다면 즉시 실행
                    ExecuteScript(filePath);
                }
            }
        }

        private void LoadSettings()
        {
            string savedPath = Properties.Settings.Default.PythonPath;
            if (string.IsNullOrWhiteSpace(savedPath)) savedPath = "python";

            TxtPythonPath.Text = savedPath;
            ChkTerminal.IsChecked = Properties.Settings.Default.UseTerminal;
            ChkCloseOnSuccess.IsChecked = Properties.Settings.Default.CloseOnSuccess;
            UpdateResolvedPath();
        }

        private void SaveSettings()
        {
            Properties.Settings.Default.PythonPath = TxtPythonPath.Text;
            Properties.Settings.Default.UseTerminal = ChkTerminal.IsChecked ?? false;
            Properties.Settings.Default.CloseOnSuccess = ChkCloseOnSuccess.IsChecked ?? false;
            Properties.Settings.Default.Save();
            UpdateResolvedPath();
        }

        private void UpdateResolvedPath()
        {
            LblResolvedPath.Text = $"Default: {TxtPythonPath.Text}";
        }

        private void BtnBrowse_Click(object sender, RoutedEventArgs e)
        {
            OpenFileDialog openFileDialog = new OpenFileDialog();
            openFileDialog.Filter = "Executables (*.exe)|*.exe|All files (*.*)|*.*";
            if (openFileDialog.ShowDialog() == true)
            {
                TxtPythonPath.Text = openFileDialog.FileName;
                SaveSettings();
            }
        }

        private void BtnProject_Click(object sender, RoutedEventArgs e)
        {
            using (var dialog = new System.Windows.Forms.FolderBrowserDialog())
            {
                dialog.Description = "Select Project Folder";
                if (dialog.ShowDialog() == System.Windows.Forms.DialogResult.OK)
                {
                    AutoDetectAndSetPython(dialog.SelectedPath);
                }
            }
        }

        private void AutoDetectAndSetPython(string folderPath)
        {
            // Windows standard venv paths
            string[] candidates = {
                Path.Combine(folderPath, ".venv", "Scripts", "python.exe"),
                Path.Combine(folderPath, "venv", "Scripts", "python.exe"),
                Path.Combine(folderPath, "env", "Scripts", "python.exe"),
                // Fallback to bin for cross-platform formed venvs on Windows (rare but possible)
                Path.Combine(folderPath, ".venv", "bin", "python.exe"), 
            };

            string found = candidates.FirstOrDefault(File.Exists);

            if (found != null)
            {
                TxtPythonPath.Text = found;
                SetStatus($"Auto-detected venv: {found}", false);
                SaveSettings();
            }
            else
            {
                MessageBox.Show("Could not find standard virtualenv (Scripts/python.exe) in:\n" + folderPath, 
                                "No Venv Found", MessageBoxButton.OK, MessageBoxImage.Warning);
            }
        }

        private void DropZone_DragEnter(object sender, DragEventArgs e)
        {
            if (e.Data.GetDataPresent(DataFormats.FileDrop)) e.Effects = DragDropEffects.Copy;
        }

        private void DropZone_Drop(object sender, DragEventArgs e)
        {
            if (e.Data.GetDataPresent(DataFormats.FileDrop))
            {
                string[] files = (string[])e.Data.GetData(DataFormats.FileDrop);
                if (files != null && files.Length > 0)
                {
                    string fileOrDir = files[0];
                    if (Directory.Exists(fileOrDir))
                    {
                        AutoDetectAndSetPython(fileOrDir);
                    }
                    else if (Path.GetExtension(fileOrDir).ToLower() == ".py") 
                    {
                        ExecuteScript(fileOrDir);
                    }
                    else 
                    {
                        SetStatus("Error: Only .py files or Project folders supported.", true);
                    }
                }
            }
        }

        private (string path, bool forceTerminal) ScanPqrHeader(string filePath)
        {
            try
            {
                var lines = File.ReadLines(filePath).Take(20);
                foreach (var line in lines)
                {
                    string trimmed = line.Trim();
                    if (trimmed.StartsWith("#pqr", StringComparison.OrdinalIgnoreCase))
                    {
                        // Remove #pqr and parse the rest
                        string argsLine = trimmed.Substring(4).Trim();
                        // Split by semicolon
                        var parts = argsLine.Split(';');
                        
                        string foundPath = null;
                        bool forceTerminal = false;

                        foreach (var part in parts)
                        {
                            var kv = part.Trim().Split('=');
                            if (kv.Length == 2)
                            {
                                string key = kv[0].Trim().ToLower();
                                string value = kv[1].Trim();

                                if (key == "win")
                                {
                                    foundPath = value;
                                }
                                else if (key == "term")
                                {
                                    bool.TryParse(value, out forceTerminal);
                                }
                            }
                        }

                        // If we found at least a path or explicit terminal setting, return it.
                        // However, based on requirements, if #pqr is present, we might want to return what we found.
                        // The previous logic returned (remainder, forceTerminal) where remainder was the path.
                        // Now we return (foundPath, forceTerminal).
                        
                        if (foundPath != null || forceTerminal)
                        {
                             return (foundPath, forceTerminal);
                        }
                    }
                }
            }
            catch (Exception ex) { Debug.WriteLine(ex.Message); }
            return (null, false);
        }

        private async void ExecuteScript(string scriptPath)
        {
            SaveSettings();
            string interpreter = TxtPythonPath.Text;
            bool useTerminal = ChkTerminal.IsChecked ?? false;
            string workingDir = Path.GetDirectoryName(scriptPath);

            var pqr = ScanPqrHeader(scriptPath);
            if (!string.IsNullOrEmpty(pqr.path)) interpreter = pqr.path;
            if (pqr.forceTerminal) useTerminal = true;

            if (useTerminal) RunInTerminal(interpreter, scriptPath, workingDir);
            else await RunInBackground(interpreter, scriptPath, workingDir);
        }

        private void RunInTerminal(string interpreter, string scriptPath, string workingDir)
        {
            try
            {
                SetStatus($"Launching in CMD...\nUsing: {interpreter}", false);
                string cmdArgs = $"/k \"cd /d \"{workingDir}\" && \"{interpreter}\" \"{scriptPath}\" && echo. && echo Exit Code: %ERRORLEVEL% && pause && exit\"";

                ProcessStartInfo psi = new ProcessStartInfo("cmd.exe", cmdArgs) { UseShellExecute = true };
                Process.Start(psi);

                SetStatus("Launched in CMD successfully.", false);
                if (ChkCloseOnSuccess.IsChecked == true) Application.Current.Shutdown();
            }
            catch (Exception ex) { SetStatus($"Error: {ex.Message}", true); }
        }

        private async Task RunInBackground(string interpreter, string scriptPath, string workingDir)
        {
            LoadingOverlay.Visibility = Visibility.Visible;
            SetStatus($"Running...\nUsing: {interpreter}", false);

            try
            {
                var result = await Task.Run(() =>
                {
                    ProcessStartInfo psi = new ProcessStartInfo
                    {
                        FileName = interpreter,
                        Arguments = $"\"{scriptPath}\"",
                        WorkingDirectory = workingDir,
                        RedirectStandardOutput = true,
                        RedirectStandardError = true,
                        UseShellExecute = false,
                        CreateNoWindow = true,
                        StandardOutputEncoding = Encoding.UTF8,
                        StandardErrorEncoding = Encoding.UTF8
                    };

                    using (Process process = new Process { StartInfo = psi })
                    {
                        StringBuilder output = new StringBuilder();
                        StringBuilder error = new StringBuilder();
                        process.OutputDataReceived += (s, e) => { if (e.Data != null) output.AppendLine(e.Data); };
                        process.ErrorDataReceived += (s, e) => { if (e.Data != null) error.AppendLine(e.Data); };

                        process.Start();
                        process.BeginOutputReadLine();
                        process.BeginErrorReadLine();
                        process.WaitForExit();

                        return (ExitCode: process.ExitCode, Output: output.ToString(), Error: error.ToString());
                    }
                });

                LoadingOverlay.Visibility = Visibility.Collapsed;
                if (result.ExitCode == 0)
                {
                    string msg = string.IsNullOrWhiteSpace(result.Output) ? "Success (No Output)" : result.Output;
                    SetStatus($"Success:\n{msg}", false);
                    if (ChkCloseOnSuccess.IsChecked == true)
                    {
                        await Task.Delay(1000);
                        Application.Current.Shutdown();
                    }
                }
                else
                {
                    string msg = string.IsNullOrWhiteSpace(result.Error) ? $"Exit Code {result.ExitCode}" : result.Error;
                    SetStatus($"Failed:\n{msg}", true);
                }
            }
            catch (Exception ex)
            {
                LoadingOverlay.Visibility = Visibility.Collapsed;
                SetStatus($"Error: {ex.Message}", true);
            }
        }

        private void SetStatus(string message, bool isError)
        {
            TxtStatus.Text = message;
            TxtStatusIcon.Text = isError ? "⚠" : "ℹ";
            TxtStatusIcon.Foreground = isError ? Brushes.Red : Brushes.Blue;
        }

        protected override void OnClosed(EventArgs e)
        {
            SaveSettings();
            base.OnClosed(e);
        }
    }
}