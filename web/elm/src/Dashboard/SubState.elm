module Dashboard.SubState exposing (..)

import Dashboard.Details as Details
import Monocle.Optional
import Monocle.Lens
import MonocleHelpers exposing (..)
import Time exposing (Time)


type alias SubState =
    { details : Maybe Details.Details
    , hideFooter : Bool
    , hideFooterCounter : Time
    , csrfToken : String
    }


detailsOptional : Monocle.Optional.Optional SubState Details.Details
detailsOptional =
    Monocle.Optional.Optional .details (\d ss -> { ss | details = Just d })


detailsLens : Monocle.Lens.Lens SubState (Maybe Details.Details)
detailsLens =
    Monocle.Lens.Lens .details (\d ss -> { ss | details = d })


hideFooterLens : Monocle.Lens.Lens SubState Bool
hideFooterLens =
    Monocle.Lens.Lens .hideFooter (\hf ss -> { ss | hideFooter = hf })


hideFooterCounterLens : Monocle.Lens.Lens SubState Time.Time
hideFooterCounterLens =
    Monocle.Lens.Lens .hideFooterCounter (\c ss -> { ss | hideFooterCounter = c })


tick : Time.Time -> SubState -> SubState
tick now =
    (detailsOptional =|> Details.nowLens).set now
        >> (\ss -> (hideFooterCounterLens.get ss |> updateFooter) ss)


showFooter : SubState -> SubState
showFooter =
    hideFooterLens.set False >> hideFooterCounterLens.set 0


updateFooter : Time.Time -> SubState -> SubState
updateFooter counter =
    if counter + Time.second > 5 * Time.second then
        hideFooterLens.set True
    else
        hideFooterCounterLens.set (counter + Time.second)
