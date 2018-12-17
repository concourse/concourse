module Dashboard.SubState exposing (..)

import Dashboard.Group as Group
import Monocle.Lens
import Time exposing (Time)


type alias SubState =
    { hideFooter : Bool
    , hideFooterCounter : Time
    , now : Time
    , dragState : Group.DragState
    , dropState : Group.DropState
    , showHelp : Bool
    }


hideFooterLens : Monocle.Lens.Lens SubState Bool
hideFooterLens =
    Monocle.Lens.Lens .hideFooter (\hf ss -> { ss | hideFooter = hf })


hideFooterCounterLens : Monocle.Lens.Lens SubState Time.Time
hideFooterCounterLens =
    Monocle.Lens.Lens .hideFooterCounter (\c ss -> { ss | hideFooterCounter = c })


tick : Time.Time -> SubState -> SubState
tick now substate =
    { substate | now = now } |> updateFooter substate.hideFooterCounter


showFooter : SubState -> SubState
showFooter =
    hideFooterLens.set False >> hideFooterCounterLens.set 0


updateFooter : Time.Time -> SubState -> SubState
updateFooter counter =
    if counter + Time.second > 5 * Time.second then
        hideFooterLens.set True
    else
        hideFooterCounterLens.set (counter + Time.second)
