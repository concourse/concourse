module Dashboard.Msgs exposing (Msg(..), fromDashboardMsg)

import Concourse.Cli as Cli
import Dashboard.Models as Models
import Keyboard
import Time
import TopBar.Msgs as NTB
import Window


type Msg
    = ClockTick Time.Time
    | AutoRefresh Time.Time
    | ShowFooter
    | KeyPressed Keyboard.KeyCode
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
    | FromTopBar NTB.Msg


fromDashboardMsg : Msg -> NTB.Msg
fromDashboardMsg msg =
    case msg of
        KeyPressed k ->
            NTB.KeyPressed k

        ResizeScreen s ->
            NTB.ResizeScreen s

        FromTopBar m ->
            m

        _ ->
            NTB.Noop
