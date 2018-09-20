module Dashboard.Details exposing (..)

import Dashboard.Group as Group
import Monocle.Lens
import Time exposing (Time)


type alias Details =
    { now : Time
    , dragState : Group.DragState
    , dropState : Group.DropState
    , showHelp : Bool
    }


nowLens : Monocle.Lens.Lens Details Time.Time
nowLens =
    Monocle.Lens.Lens .now (\t ss -> { ss | now = t })


dragStateLens : Monocle.Lens.Lens Details Group.DragState
dragStateLens =
    Monocle.Lens.Lens .dragState (\ds ss -> { ss | dragState = ds })


dropStateLens : Monocle.Lens.Lens Details Group.DropState
dropStateLens =
    Monocle.Lens.Lens .dropState (\ds ss -> { ss | dropState = ds })


toggleHelp : Details -> Details
toggleHelp details =
    { details | showHelp = not details.showHelp }
