; --------------------------------------------------------------------------------
; Inno Setup Script for Logs Exporter
; --------------------------------------------------------------------------------

[Setup]
AppId={{YOUR-GUID-HERE}}
AppName=LogsExporter
AppVersion=1.0
DefaultDirName={pf}\LogsExporter
DefaultGroupName=LogsExporter
OutputBaseFilename=LogsExporterSetup
WizardStyle=modern
Compression=lzma
SolidCompression=yes
DisableProgramGroupPage=yes
PrivilegesRequired=admin
VersionInfoVersion=1.0.0
VersionInfoCompany=YourCompany
VersionInfoDescription=Logs Exporter Installation

[Files]
; Correct Go-built exporter binary
Source: "C:\projects\Logs_exporter\dist\windows\logs_exporter.exe"; DestDir: "{app}"; DestName: "logs_exporter.exe"; Flags: ignoreversion

; C# watcher binary
Source: "C:\projects\Logs_exporter\dist\windows\FolderMonitor.exe"; DestDir: "{app}"; Flags: ignoreversion

; WinDivert files
Source: "C:\projects\Logs_exporter\dist\windows\WinDivert.dll"; DestDir: "{app}"; Flags: ignoreversion
Source: "C:\projects\Logs_exporter\dist\windows\WinDivert64.sys"; DestDir: "{app}"; Flags: ignoreversion

; config.json template
Source: "C:\projects\Logs_exporter\dist\windows\config.json"; DestDir: "{app}"; Flags: ignoreversion skipifsourcedoesntexist


[Icons]
Name: "{group}\Logs Exporter"; Filename: "{app}\logs_exporter.exe"
Name: "{group}\Uninstall Logs Exporter"; Filename: "{uninstallexe}"

[Code]
// Function to check if a Windows service exists
function ServiceExists(ServiceName: string): Boolean;
var
  ExitCode: Integer;
begin
  Result := False;
  if Exec('sc', 'query "' + ServiceName + '"', '', SW_HIDE, ewWaitUntilTerminated, ExitCode) then
    Result := (ExitCode = 0);
end;

var
  QueryPage: TInputQueryWizardPage;
  CbPushMode, CbWinService: TCheckBox;

procedure InitializeWizard();
var
  YPos: Integer;
begin
  QueryPage := CreateInputQueryPage(
    wpWelcome,
    'Logs Exporter Setup',
    'Configure Logs Exporter',
    'Set the following configuration for Logs Exporter.'
  );

  QueryPage.Add('Folder to watch:', False);
  QueryPage.Values[0] := 'C:\logs';

  QueryPage.Add('NATS URL (for push mode):', False);
  QueryPage.Values[1] := 'nats://127.0.0.1:4222';

  QueryPage.Add('Subject prefix (default is "metrics"):', False);
  QueryPage.Values[2] := 'metrics';

  QueryPage.Add('Port (for HTTP server):', False);
  QueryPage.Values[3] := '9182';

  YPos := QueryPage.Edits[3].Top + QueryPage.Edits[3].Height + 15;

  CbPushMode := TCheckBox.Create(WizardForm);
  CbPushMode.Parent := QueryPage.Surface;
  CbPushMode.Top := YPos;
  CbPushMode.Left := QueryPage.Edits[0].Left;
  CbPushMode.Width := 300;
  CbPushMode.Caption := 'Enable push mode (NATS JetStream)';

  CbWinService := TCheckBox.Create(WizardForm);
  CbWinService.Parent := QueryPage.Surface;
  CbWinService.Top := YPos + 25;
  CbWinService.Left := CbPushMode.Left;
  CbWinService.Width := 300;
  CbWinService.Caption := 'Install as Windows service';
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if CurPageID = QueryPage.ID then
  begin
    if Trim(QueryPage.Values[3]) = '' then
    begin
      MsgBox('Please enter a port number.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    if CbPushMode.Checked then
    begin
      if Trim(QueryPage.Values[1]) = '' then
      begin
        MsgBox('Please enter the NATS URL.', mbError, MB_OK);
        Result := False;
        Exit;
      end;
    end;
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  PushMode, ServiceMode: Boolean;
  ExePath, InstallParams, ParamStr: String;
  ExitCode, ExecCode: Integer;
  NatsURL, SubjectPrefix, Port, WatchDir, ModeStr, ConfigPath, ConfigContent, HostName: String;
begin
  if CurStep = ssPostInstall then
  begin
    PushMode := CbPushMode.Checked;
    ServiceMode := CbWinService.Checked;
    ExePath := ExpandConstant('{app}\logs_exporter.exe');

    // Retrieve values from the custom page
    WatchDir := QueryPage.Values[0];
    NatsURL := QueryPage.Values[1];
    SubjectPrefix := QueryPage.Values[2];
    Port := QueryPage.Values[3];

    ModeStr := 'scrape';
    if PushMode then
      ModeStr := 'push';

    // Build CLI params if running standalone
    ParamStr := '-port "' + Port + '" -watch_dir "' + WatchDir + '"';
    if PushMode then
      ParamStr := '-push -nats_url "' + NatsURL + '" -port "' + Port + '" -watch_dir "' + WatchDir + '"';

    // Write config.json
    HostName := GetComputerNameString();
    ConfigPath := ExpandConstant('{app}\config.json');
    ConfigContent :=
      '{' + #13#10 +
      '  "port": "' + Port + '",' + #13#10 +
      '  "system_name": "' + HostName + '",' + #13#10 +
      '  "nats_url": "' + NatsURL + '",' + #13#10 +
      '  "mode": "' + ModeStr + '",' + #13#10 +
      '  "watch_dir": "' + WatchDir + '",' + #13#10 +
      '  "watch_window_ms": 2000,' + #13#10 +
      '  "netflow_interfaces": []' + #13#10 +
      '}';

    SaveStringToFile(ConfigPath, ConfigContent, False);

    // Install as service or run standalone
    if ServiceMode then
    begin
      InstallParams := '--service install';
      if Exec(ExePath, InstallParams, '', SW_HIDE, ewWaitUntilTerminated, ExitCode) then
      begin
        if not Exec(ExePath, '--service start', '', SW_HIDE, ewWaitUntilTerminated, ExitCode) then
          MsgBox('Failed to start service.', mbError, MB_OK);
      end
      else
        MsgBox('Service installation failed.', mbError, MB_OK);
    end
    else
    begin
      if not ShellExec('open', ExePath, ParamStr, '', SW_SHOWNORMAL, ewNoWait, ExecCode) then
        MsgBox('Failed to launch Logs Exporter.', mbError, MB_OK);
    end;
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  StopCode, UninstallCode: Integer;
  ExePath, LogPath: String;
begin
  if CurUninstallStep = usUninstall then
  begin
    ExePath := ExpandConstant('{app}\logs_exporter.exe');

    if ServiceExists('LogsExporterService') then
    begin
      if not Exec(ExePath, '--service stop', '', SW_HIDE, ewWaitUntilTerminated, StopCode) then
        MsgBox('Failed to stop service.', mbError, MB_OK);
      if not Exec(ExePath, '--service uninstall', '', SW_HIDE, ewWaitUntilTerminated, UninstallCode) then
        MsgBox('Failed to uninstall service.', mbError, MB_OK);
    end;

    // Prompt user to delete logs
    if MsgBox('Do you want to delete the generated log files?', mbConfirmation, MB_YESNO) = IDYES then
    begin
      LogPath := ExpandConstant('{app}\logs_exporter_debug.log');
      if FileExists(LogPath) then
        DeleteFile(LogPath);

      // Additional logs can be deleted here if needed
    end;
  end;
end;
