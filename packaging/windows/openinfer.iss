; OpenInfer Studio — Inno Setup installer script.
; Built by packaging/windows/build-installer.ps1
; Always pass absolute /DMyAppDir and /DMyOutputDir (Inno resolves relative
; paths against this .iss file's directory, not the caller's cwd).

#ifndef MyAppVersion
  #define MyAppVersion "1.0.0"
#endif
#ifndef MyAppDir
  #error MyAppDir must be set to an absolute payload path via /DMyAppDir=...
#endif
#ifndef MyOutputDir
  #error MyOutputDir must be set to an absolute output path via /DMyOutputDir=...
#endif

[Setup]
AppId={{A7E3C9F1-4B2D-4E8A-9C1F-8D2E6B5A4C30}
AppName=OpenInfer Studio
AppVersion={#MyAppVersion}
AppPublisher=OpenInfer
AppPublisherURL=https://github.com/32bitx64bit/OpenInfer
DefaultDirName={autopf}\OpenInfer Studio
DefaultGroupName=OpenInfer Studio
DisableProgramGroupPage=yes
OutputDir={#MyOutputDir}
OutputBaseFilename=OpenInferStudio-{#MyAppVersion}-windows-x86_64-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayIcon={app}\openinfer-studio.exe

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#MyAppDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\OpenInfer Studio"; Filename: "{app}\openinfer-studio.exe"
Name: "{group}\{cm:UninstallProgram,OpenInfer Studio}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\OpenInfer Studio"; Filename: "{app}\openinfer-studio.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\openinfer-studio.exe"; Description: "{cm:LaunchProgram,OpenInfer Studio}"; Flags: nowait postinstall skipifsilent
