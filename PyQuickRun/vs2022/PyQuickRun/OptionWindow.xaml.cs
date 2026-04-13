// Created by DINKIssTyle on 2026. Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

using System.Windows;
using System.Windows.Input;

namespace PyQuickRun
{
    public partial class OptionWindow : Window
    {
        public bool RunRequested { get; private set; }
        public bool SaveRequested { get; private set; }
        public string Category => TxtCategory.Text;
        public bool UseTerminal => ChkTerminal.IsChecked ?? false;
        public bool CloseOnSuccess => ChkCloseOnSuccess.IsChecked ?? false;

        public OptionWindow(bool defaultCloseOnSuccess)
        {
            InitializeComponent();
            ChkCloseOnSuccess.IsChecked = defaultCloseOnSuccess;
            ChkTerminal.IsChecked = false; // Default to unchecked as requested

            this.KeyDown += (s, e) =>
            {
                if (Keyboard.Modifiers == ModifierKeys.Control)
                {
                    if (e.Key == Key.D)
                    {
                        RunNow();
                        e.Handled = true;
                    }
                    else if (e.Key == Key.S)
                    {
                        SaveAndRun();
                        e.Handled = true;
                    }
                }
            };
        }

        private void BtnRunNow_Click(object sender, RoutedEventArgs e) => RunNow();
        private void BtnSaveRun_Click(object sender, RoutedEventArgs e) => SaveAndRun();

        private void RunNow()
        {
            RunRequested = true;
            this.DialogResult = true;
            this.Close();
        }

        private void SaveAndRun()
        {
            SaveRequested = true;
            this.DialogResult = true;
            this.Close();
        }
    }
}
