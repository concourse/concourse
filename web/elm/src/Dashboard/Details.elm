module Dashboard.Details exposing (Details, dragStateLens, dropStateLens, nowLens)

import Dashboard.Models as Models
import Monocle.Lens
import Time exposing (Time)


type alias Details r =
    { r
        | now : Time
        , dragState : Models.DragState
        , dropState : Models.DropState
    }


nowLens : Monocle.Lens.Lens (Details r) Time.Time
nowLens =
    Monocle.Lens.Lens .now (\t ss -> { ss | now = t })


dragStateLens : Monocle.Lens.Lens (Details r) Models.DragState
dragStateLens =
    Monocle.Lens.Lens .dragState (\ds ss -> { ss | dragState = ds })


dropStateLens : Monocle.Lens.Lens (Details r) Models.DropState
dropStateLens =
    Monocle.Lens.Lens .dropState (\ds ss -> { ss | dropState = ds })
