module Dashboard.Msgs exposing (Msg(..), fromDashboardMsg)

import Concourse.Cli as Cli
import Dashboard.Models as Models
import Keyboard
import NewTopBar.Msgs as NTB
import Time
import Window


type Msg
    = ClockTick Time.Time
    | AutoRefresh Time.Time
    | ShowFooter
    | KeyPressed Keyboard.KeyCode
    | KeyDowns Keyboard.KeyCode
    | DragStart String Int
    | DragOver String Int
    | DragEnd
    | Tooltip String String
    | TooltipHd String String
    | TogglePipelinePaused Models.Pipeline
    | PipelineButtonHover (Maybe Models.Pipeline)
    | CliHover (Maybe Cli.Cli)
    | TopCliHover (Maybe Cli.Cli)
    | ResizeScreen Window.Size
    | LogIn
    | LogOut
    | FilterMsg String
    | FocusMsg
    | BlurMsg
    | SelectMsg Int
    | ToggleUserMenu
    | ShowSearchInput
    | FromTopBar NTB.Msg


fromDashboardMsg : Msg -> NTB.Msg
fromDashboardMsg msg =
    case msg of
        LogIn ->
            NTB.LogIn

        LogOut ->
            NTB.LogOut

        FilterMsg s ->
            NTB.FilterMsg s

        FocusMsg ->
            NTB.FocusMsg

        BlurMsg ->
            NTB.BlurMsg

        SelectMsg i ->
            NTB.SelectMsg i

        KeyDowns k ->
            NTB.KeyDown k

        KeyPressed k ->
            NTB.KeyPressed k

        ToggleUserMenu ->
            NTB.ToggleUserMenu

        ShowSearchInput ->
            NTB.ShowSearchInput

        ResizeScreen s ->
            NTB.ResizeScreen s

        FromTopBar m ->
            m

        _ ->
            NTB.Noop
