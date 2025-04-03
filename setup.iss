;---------------------------------------------------------------------
; Fully Corrected setup.iss (No warnings)
;---------------------------------------------------------------------

[Setup]
AppName=Logs Exporter
AppVersion=1.0
DefaultDirName={commonpf}\LogsExporter
DefaultGroupName=Logs Exporter
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputBaseFilename=LogsExporterSetup
Compression=lzma
SolidCompression=yes

[Files]
; Adjust these paths to match your actual build outputs.
Source: "C:\projects\Logs_exporter\bin\windows_amd64\windows_exporter.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "C:\projects\Logs_exporter\config.json"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
; Icon for normal usage
Name: "{group}\Logs Exporter"; Filename: "{app}\windows_exporter.exe"; \
    IconIndex: 0; Tasks: installnormal

[Tasks]
Name: "installservice"; Description: "Install Logs Exporter as a Windows Service"; GroupDescription: "Installation Mode:"
Name: "installnormal";  Description: "Install as a standard application (run manually)"; GroupDescription: "Installation Mode:"

[Run]
; Install and start service if selected
Filename: "{app}\windows_exporter.exe"; Parameters: "-service install"; \
    Flags: nowait runhidden; Check: WizardIsTaskSelected('installservice')
Filename: "{app}\windows_exporter.exe"; Parameters: "-service start"; \
    Flags: nowait runhidden; Check: WizardIsTaskSelected('installservice')

; Launch after installation if selected
Filename: "{app}\windows_exporter.exe"; Description: "Launch Logs Exporter now"; \
    Flags: nowait postinstall skipifsilent; Check: WizardIsTaskSelected('installnormal')

[UninstallRun]
; Stop and uninstall the service if it was installed previously
Filename: "{app}\windows_exporter.exe"; Parameters: "-service stop"; \
    Flags: nowait runhidden; Check: WizardIsTaskSelected('installservice'); RunOnceId: "StopService"
Filename: "{app}\windows_exporter.exe"; Parameters: "-service uninstall"; \
    Flags: nowait runhidden; Check: WizardIsTaskSelected('installservice'); RunOnceId: "UninstallService"
