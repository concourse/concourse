module Dashboard.Msgs exposing (Msg(..), fromDashboardMsg)

import Concourse.Cli as Cli
import Dashboard.Models as Models
import Keyboard
import Time
import TopBar.Msgs
import Window


type Msg
    = ClockTick Time.Time
    | AutoRefresh
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
    | WindowResized Window.Size
    | FromTopBar TopBar.Msgs.Msg


fromDashboardMsg : Msg -> TopBar.Msgs.Msg
fromDashboardMsg msg =
    case msg of
        KeyPressed k ->
            TopBar.Msgs.KeyPressed k

        WindowResized s ->
            TopBar.Msgs.WindowResized s

        FromTopBar m ->
            m

        _ ->
            TopBar.Msgs.Noop
