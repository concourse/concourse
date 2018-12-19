module Dashboard.SubState exposing (..)

import Dashboard.Group as Group
import Time exposing (Time)


type alias SubState =
    { now : Time
    , dragState : Group.DragState
    , dropState : Group.DropState
    }


tick : Time.Time -> SubState -> SubState
tick now substate =
    { substate | now = now }
