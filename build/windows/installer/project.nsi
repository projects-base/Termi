Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## Include the wails tools
####
!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"
!include "WordFunc.nsh"
!include "LogicLib.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

# Finish page: offer to launch Termi after install.
#
# Through a function rather than directly, because this installer runs elevated
# and anything it starts inherits that. A terminal is the worst possible thing
# to hand an administrator token by accident: every command typed into it would
# run elevated, with no sign that anything was unusual. LaunchAsUser goes
# through explorer.exe, which is running as the logged-in user.
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_FUNCTION LaunchAsUser
!define MUI_FINISHPAGE_RUN_TEXT "Launch ${INFO_PRODUCTNAME}"

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}" # Default installing folder ($PROGRAMFILES is Program Files folder).
# An upgrade must land on top of the existing install, not beside it. Without
# this, a silent update from inside the app would always use the default folder,
# leaving anyone who installed elsewhere with two copies.
InstallDirRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${UNINST_KEY_NAME}" "InstallLocation"
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture

   # Windows will not replace a running exe, so an upgrade over an open Termi
   # leaves the old binary in place while everything else updates. Say so
   # rather than letting it half-apply.
   #
   # Skipped when silent: the in-app updater closes Termi before running this,
   # and there would be no one to answer a message box anyway.
   IfSilent done_running_check
   nsExec::ExecToStack 'cmd /c tasklist /FI "IMAGENAME eq ${PRODUCT_EXECUTABLE}" /NH | find /I "${PRODUCT_EXECUTABLE}"'
   Pop $0
   Pop $1
   ${If} $0 == 0
     MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION \
       "${INFO_PRODUCTNAME} is still running.$\r$\n$\r$\nClose it first, or this \
install will not be able to replace the program itself.$\r$\n$\r$\nOK to continue \
anyway, Cancel to stop." \
       IDOK done_running_check
     Abort
   ${EndIf}
   done_running_check:
FunctionEnd

# Starts Termi as the logged-in user. explorer.exe owns the desktop and runs
# unelevated, so a process it launches is unelevated too - which is what we
# want for a terminal, even though this installer needed admin rights.
Function LaunchAsUser
  Exec '"$WINDIR\explorer.exe" "$INSTDIR\${PRODUCT_EXECUTABLE}"'
FunctionEnd

####
## Removing $INSTDIR from a PATH string, correctly.
##
## Matching on ";$INSTDIR;" rather than on the bare folder name is the point: a
## plain substring replace also eats a sibling folder, turning a PATH holding
## "C:\PF\Termi Beta" into "C:\Windows Beta" when "C:\PF\Termi" is removed.
##
## One occurrence per pass, so duplicates sitting next to each other cannot
## hide behind an overlap. The E flag is load-bearing - it is what makes
## WordReplace set the error flag when there is nothing left to replace, and
## without it this loop never ends.
##
## In:  $0 = the current PATH
## Out: $1 = that PATH with every copy of $INSTDIR taken out
####
!macro PathWithoutInstDir UN
Function ${UN}PathWithoutInstDir
  ; Pad both ends so the first and last entries are delimited like the rest.
  StrCpy $1 ";$0;"

  path_loop:
    ClearErrors
    ${WordReplace} "$1" ";$INSTDIR;" ";" "E+1" $2
    IfErrors path_done
    StrCpy $1 "$2"
    Goto path_loop
  path_done:

  ; Drop the padding.
  StrLen $3 "$1"
  IntOp $3 $3 - 2
  StrCpy $1 "$1" $3 1

  ; A PATH that already ended in ";" would otherwise be left with ";;".
  ${If} $1 != ""
    StrCpy $4 "$1" "" -1
    ${If} $4 == ";"
      StrCpy $1 "$1" -1
    ${EndIf}
  ${EndIf}
FunctionEnd
!macroend

; The installer and the uninstaller need their own copies; NSIS keeps the two
; sets of functions apart by the "un." prefix.
!insertmacro PathWithoutInstDir ""
!insertmacro PathWithoutInstDir "un."

Section
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files

    # --- Set PowerShell Execution Policy to RemoteSigned for CurrentUser ---
    # This prevents the "running scripts is disabled" error inside Termi
    DetailPrint "Configuring PowerShell execution policy..."
    nsExec::ExecToLog 'powershell.exe -NoProfile -Command "Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force"'

    # --- Create Start Menu shortcuts ---
    CreateDirectory "$SMPROGRAMS\${INFO_PRODUCTNAME}"
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}\Uninstall ${INFO_PRODUCTNAME}.lnk" "$INSTDIR\uninstall.exe"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    # --- Add to PATH so 'termi' can be launched from any terminal ---
    DetailPrint "Adding ${INFO_PRODUCTNAME} to system PATH..."
    SetRegView 64
    ReadRegStr $0 HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "Path"
    # Take out any entry from a previous install before appending, so that
    # installing over an existing copy cannot stack duplicates. It did: three
    # installs left three identical entries, and an in-app updater that
    # installs on every release would add one more each time.
    Call PathWithoutInstDir
    ${If} $1 == ""
      StrCpy $5 "$INSTDIR"
    ${Else}
      StrCpy $5 "$1;$INSTDIR"
    ${EndIf}
    WriteRegExpandStr HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "Path" "$5"
    # Notify the system about environment change
    SendMessage 0xFFFF 0x001A 0 "STR:Environment" /TIMEOUT=5000

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller

    # wails_tools.nsh writes the rest of the uninstall entry but not this, and
    # both Windows and the in-app updater look for it.
    SetRegView 64
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${UNINST_KEY_NAME}" "InstallLocation" "$INSTDIR"
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    RMDir /r $INSTDIR

    # --- Remove Start Menu shortcuts ---
    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}\${INFO_PRODUCTNAME}.lnk"
    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}\Uninstall ${INFO_PRODUCTNAME}.lnk"
    RMDir "$SMPROGRAMS\${INFO_PRODUCTNAME}"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    # --- Remove from PATH ---
    SetRegView 64
    ReadRegStr $0 HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "Path"
    Call un.PathWithoutInstDir
    WriteRegExpandStr HKLM "SYSTEM\CurrentControlSet\Control\Session Manager\Environment" "Path" "$1"
    SendMessage 0xFFFF 0x001A 0 "STR:Environment" /TIMEOUT=5000

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
