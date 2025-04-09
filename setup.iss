; --------------------------------------------------------------------------------
; Inno Setup Script for Logs Exporter
; --------------------------------------------------------------------------------

[Setup]
AppName=LogsExporter
AppVersion=1.0
DefaultDirName={pf}\LogsExporter
DefaultGroupName=LogsExporter
OutputBaseFilename=LogsExporterSetup
WizardStyle=modern
Compression=lzma
SolidCompression=yes
DisableDirPage=yes
DisableProgramGroupPage=yes
PrivilegesRequired=admin
VersionInfoVersion=1.0.0
VersionInfoCompany=YourCompany
VersionInfoDescription=Logs Exporter Installation
; LicenseFile=license.txt  ; Optional

[Files]
Source: "C:\projects\Logs_exporter\bin\windows_amd64\windows_exporter.exe"; DestDir: "{app}"; DestName: "logs_exporter.exe"; Flags: ignoreversion
Source: "C:\projects\Logs_exporter\config.json"; DestDir: "{app}"; Flags: ignoreversion

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

  // Add input fields and set default values
  QueryPage.Add('NATS URL (for push mode):', False);
  QueryPage.Values[0] := 'nats://127.0.0.1:4222';

  QueryPage.Add('Subject prefix (default is "metrics"):', False);
  QueryPage.Values[1] := 'metrics';

  QueryPage.Add('Port (for HTTP server):', False);
  QueryPage.Values[2] := '9182';

  YPos := QueryPage.Edits[2].Top + QueryPage.Edits[2].Height + 15;

  // Create checkboxes with proper positioning
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
  if CurPageID = QueryPage.ID then
  begin
    // Validate required fields
    if Trim(QueryPage.Values[2]) = '' then
    begin
      MsgBox('Please enter a port number.', mbError, MB_OK);
      Result := False;
      Exit;
    end;

    if CbPushMode.Checked then
    begin
      if Trim(QueryPage.Values[0]) = '' then
      begin
        MsgBox('Please enter the NATS URL.', mbError, MB_OK);
        Result := False;
        Exit;
      end;
      if Trim(QueryPage.Values[1]) = '' then
      begin
        MsgBox('Please enter the Subject prefix.', mbError, MB_OK);
        Result := False;
        Exit;
      end;
    end;
  end;
  Result := True;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  PushMode, ServiceMode: Boolean;
  ExePath, InstallParams, ParamStr: String;
  ExitCode, ExecCode: Integer;
  NatsURL, SubjectPrefix, Port: String;
begin
  if CurStep = ssPostInstall then
  begin
    PushMode := CbPushMode.Checked;
    ServiceMode := CbWinService.Checked;
    ExePath := ExpandConstant('{app}\logs_exporter.exe');

    // Retrieve values from the custom page; these are only for standalone mode.
    NatsURL := QueryPage.Values[0];
    SubjectPrefix := QueryPage.Values[1];
    Port := QueryPage.Values[2];

    // Build parameters for standalone run
    ParamStr := Format('-port "%s"', [Port]);
    if PushMode then
      ParamStr := Format('-push -nats_url "%s" -nats_subject "%s" -port "%s"', [NatsURL, SubjectPrefix, Port]);

    if ServiceMode then
    begin
      // For service installation, call with only the service flag.
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
  ExePath: String;
begin
  if CurUninstallStep = usUninstall then
  begin
    ExePath := ExpandConstant('{app}\logs_exporter.exe');
    
    // Stop service if it exists
    if ServiceExists('LogsExporterService') then
    begin
      if not Exec(ExePath, '--service stop', '', SW_HIDE, ewWaitUntilTerminated, StopCode) then
        MsgBox('Failed to stop service.', mbError, MB_OK);
      if not Exec(ExePath, '--service uninstall', '', SW_HIDE, ewWaitUntilTerminated, UninstallCode) then
        MsgBox('Failed to uninstall service.', mbError, MB_OK);
    end;
  end;
end;
