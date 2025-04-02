[Setup]
AppName=Logs Exporter
AppVersion=1.0
DefaultDirName={pf}\LogsExporter
DefaultGroupName=Logs Exporter
OutputBaseFilename=LogsExporterSetup
Compression=lzma
SolidCompression=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "bin\windows_amd64\windows_exporter.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "config.json";                          DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Logs Exporter";            Filename: "{app}\windows_exporter.exe"
Name: "{group}\Uninstall Logs Exporter";  Filename: "{uninstallexe}"

[Run]
Filename: "{app}\windows_exporter.exe"; Description: "Run Logs Exporter"; \
  Flags: nowait postinstall skipifsilent

[Code]
var
  PortPage: TInputQueryWizardPage;

procedure InitializeWizard;
begin
  // Create a simple page asking the user for the port
  PortPage := CreateInputQueryPage(
    wpSelectDir,
    'Configuration',
    'Configuration Options',
    'Please specify the port for Logs Exporter:'
  );
  // Add one edit field labeled “Port:”
  PortPage.Add('Port:', False);             // Add field (not password)
  PortPage.Values[0] := '9182';             // Set default port value
end;

// We only need CurStepChanged to rewrite config.json after install
procedure CurStepChanged(CurStep: TSetupStep);
var
  PortValue: string;
  ConfigFile: string;
  ConfigFileContent: string;
  SL: TStringList;
begin
  if CurStep = ssPostInstall then
  begin
    PortValue := PortPage.Values[0];
    ConfigFileContent :=
      '{'#13#10 +
      '  "port": "' + PortValue + '"'#13#10 +
      '}';

    ConfigFile := ExpandConstant('{app}\config.json');

    // Write out the JSON via a TStringList
    SL := TStringList.Create;
    try
      SL.Text := ConfigFileContent;
      SL.SaveToFile(ConfigFile);
    finally
      SL.Free;
    end;
  end;
end;
