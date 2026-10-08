; Inno Setup script for Task Activity Manager.
;
; Driven by scripts/release-app.ps1, which compiles it with ISCC and passes the
; version + build paths as /D defines:
;   ISCC /DAppVersion=0.1.0 /DSourceDir=<build\bin> /DOutputDir=<dist> \
;        [/DWebView2Bootstrapper=<path to MicrosoftEdgeWebview2Setup.exe>] installer.iss
;
; The defines have sensible fallbacks so the script also compiles standalone
; (e.g. opening it in the Inno Setup IDE) after a `wails build`.
;
; This is the twin of xtm/build/windows/installer/installer.iss. The two are
; deliberately separate files rather than one shared script: AppId must differ
; between them and must never change once an app has shipped, so it belongs
; beside the app it identifies, not in a flag passed from a build script.

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif
#ifndef SourceDir
  #define SourceDir "..\..\bin"
#endif
#ifndef OutputDir
  #define OutputDir "..\..\..\dist"
#endif

#define AppName "Task Activity Manager"
#define AppPublisher "Achmad Fienan Rahardianto"
#define AppExeName "task-activity-manager.exe"
#define AppUrl "https://github.com/veenone/task-activity-manager"

[Setup]
; A stable AppId keeps upgrades and uninstall entries consistent across
; versions — do not change it once released. It is TAM's own: sharing XTM's
; would make each app's installer upgrade over the other.
AppId={{2975C214-24CD-490B-BCD2-025C9B56C581}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppUrl}
AppSupportURL={#AppUrl}
AppUpdatesURL={#AppUrl}
DefaultDirName={autopf}\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
UninstallDisplayIcon={app}\{#AppExeName}
OutputDir={#OutputDir}
OutputBaseFilename=task-activity-manager-{#AppVersion}-windows-amd64-installer
SetupIconFile=..\icon.ico
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64
; Per-machine install (Program Files) requires elevation.
PrivilegesRequired=admin

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SourceDir}\{#AppExeName}"; DestDir: "{app}"; Flags: ignoreversion
#ifdef WebView2Bootstrapper
Source: "{#WebView2Bootstrapper}"; DestDir: "{tmp}"; DestName: "MicrosoftEdgeWebview2Setup.exe"; Flags: deleteafterinstall
#endif

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExeName}"
Name: "{group}\{cm:UninstallProgram,{#AppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; Tasks: desktopicon

[Run]
#ifdef WebView2Bootstrapper
; Install the Evergreen WebView2 runtime when it isn't already present.
Filename: "{tmp}\MicrosoftEdgeWebview2Setup.exe"; Parameters: "/silent /install"; StatusMsg: "Installing Microsoft Edge WebView2 runtime…"; Check: WebView2Missing
#endif
Filename: "{app}\{#AppExeName}"; Description: "{cm:LaunchProgram,{#AppName}}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
; Remove the app's WebView2 user-data folder.
Type: filesandordirs; Name: "{localappdata}\{#AppExeName}"

[Code]
// WebView2Missing reports whether the Evergreen WebView2 runtime is absent, so
// the bootstrapper only runs when needed. The Evergreen runtime registers a
// version ("pv") under this well-known client GUID, per-machine (HKLM, under
// WOW6432Node on 64-bit) or per-user (HKCU).
function WebView2Missing(): Boolean;
var
  pv: String;
begin
  Result := not (
    RegQueryStringValue(HKLM, 'SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', pv) or
    RegQueryStringValue(HKLM, 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', pv) or
    RegQueryStringValue(HKCU, 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', pv)
  );
end;
