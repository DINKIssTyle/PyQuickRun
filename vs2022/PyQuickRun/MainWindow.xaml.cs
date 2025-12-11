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

// [중요] 프로젝트 이름에 맞게 수정됨
namespace PyQuickRun
{
    public partial class MainWindow : Window
    {
        public MainWindow()
        {
            InitializeComponent();
            LoadSettings();
        }

        private void LoadSettings()
        {
            // 이제 PyQuickRun 프로젝트 안의 Properties를 정상적으로 찾을 것입니다.
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
                    string file = files[0];
                    if (Path.GetExtension(file).ToLower() == ".py") ExecuteScript(file);
                    else SetStatus("Error: Only .py files are supported.", true);
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
                    if (trimmed.StartsWith("#pqr win", StringComparison.OrdinalIgnoreCase))
                    {
                        string remainder = trimmed.Substring("#pqr win".Length).Trim();
                        bool forceTerminal = false;

                        if (remainder.StartsWith("terminal", StringComparison.OrdinalIgnoreCase))
                        {
                            forceTerminal = true;
                            remainder = remainder.Substring("terminal".Length).Trim();
                        }
                        return (remainder, forceTerminal);
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