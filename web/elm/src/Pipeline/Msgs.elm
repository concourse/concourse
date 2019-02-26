module Pipeline.Msgs exposing (Msg(..))

import Concourse
import Keyboard
import Time exposing (Time)
import TopBar.Msgs


type Msg
    = AutoupdateVersionTicked
    | AutoupdateTimerTicked
    | HideLegendTimerTicked Time
    | ShowLegend
    | KeyPressed Keyboard.KeyCode
    | PipelineIdentifierFetched Concourse.PipelineIdentifier
    | ToggleGroup Concourse.PipelineGroup
    | SetGroups (List String)
    | FromTopBar TopBar.Msgs.Msg
