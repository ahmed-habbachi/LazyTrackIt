; Inno Setup script for the LazyTrackIt desktop app on Windows.
;
; It installs per user (no admin rights) into
; %LOCALAPPDATA%\Programs\LazyTrackIt, a folder the user can write to, so
; the app's own auto-update keeps working after it's installed this way.
;
; Build (Inno Setup 6), from this folder:
;   iscc /DAppVersion=v1.2.3 /DNumVersion=1.2.3 /DBinDir=C:\path\to\dir-with-exe lazytrackit.iss
; BinDir must hold lazytrackit-gui.exe. The installer is written to OutDir.

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif
#ifndef NumVersion
  #define NumVersion "0.0.0"
#endif
#ifndef BinDir
  #define BinDir "..\..\dist"
#endif
#ifndef OutDir
  #define OutDir "..\..\dist"
#endif

#define AppExe "lazytrackit-gui.exe"

[Setup]
; Never change AppId: it's how Windows ties upgrades and the uninstaller
; to earlier installs.
AppId={{31ECA3F6-63EC-42EC-AAD7-B436477597B3}
AppName=LazyTrackIt
AppVersion={#AppVersion}
AppVerName=LazyTrackIt {#AppVersion}
AppPublisher=LazyTrackIt
AppPublisherURL=https://github.com/ahmed-habbachi/LazyTrackIt
AppSupportURL=https://github.com/ahmed-habbachi/LazyTrackIt/issues
AppUpdatesURL=https://github.com/ahmed-habbachi/LazyTrackIt/releases
VersionInfoVersion={#NumVersion}
VersionInfoProductName=LazyTrackIt
VersionInfoDescription=LazyTrackIt installer
PrivilegesRequired=lowest
DefaultDirName={localappdata}\Programs\LazyTrackIt
; The folder is fixed: somewhere the user can't write to (like Program
; Files) would stop auto-update from working.
DisableDirPage=yes
DisableProgramGroupPage=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
SetupIconFile=lazytrackit.ico
UninstallDisplayIcon={app}\{#AppExe}
UninstallDisplayName=LazyTrackIt
WizardStyle=modern
Compression=lzma2
SolidCompression=yes
; The running app is stopped in [Code] instead: it hides in the tray when
; asked to close, so the Restart Manager can't close it.
CloseApplications=no
OutputDir={#OutDir}
OutputBaseFilename=lazytrackit-setup_{#AppVersion}_windows_amd64

[Tasks]
Name: desktopicon; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#BinDir}\{#AppExe}"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
; The same shortcut the app adds on its first run, so there's only one.
Name: "{userprograms}\LazyTrackIt"; Filename: "{app}\{#AppExe}"; WorkingDir: "{app}"; Comment: "Log your work time and check in or out"
Name: "{userdesktop}\LazyTrackIt"; Filename: "{app}\{#AppExe}"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#AppExe}"; Description: "{cm:LaunchProgram,LazyTrackIt}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
; Leftovers from the app's self-update.
Type: files; Name: "{app}\{#AppExe}.old"
Type: files; Name: "{app}\.lazytrackit-new"

[Code]
const
  RunKey = 'Software\Microsoft\Windows\CurrentVersion\Run';
  RunValue = 'lazytrackit'; { the app's "Start at login" entry }

procedure StopRunningApp();
var
  Code: Integer;
begin
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM {#AppExe}', '', SW_HIDE, ewWaitUntilTerminated, Code);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  StopRunningApp();
  Result := '';
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Cmd: String;
begin
  if CurUninstallStep = usUninstall then
  begin
    StopRunningApp();
    { Remove "Start at login" if it starts this copy of the app. }
    if RegQueryStringValue(HKCU, RunKey, RunValue, Cmd) and
       (Pos(Lowercase(ExpandConstant('{app}')), Lowercase(Cmd)) > 0) then
      RegDeleteValue(HKCU, RunKey, RunValue);
  end;
end;
