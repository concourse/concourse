module Dashboard.SubState exposing (..)

import Concourse
import Dashboard.Group as Group
import Dashboard.Details as Details
import Monocle.Optional
import Monocle.Lens
import MonocleHelpers exposing (..)
import Time exposing (Time)


type alias SubState =
    { details : Maybe Details.Details
    , teamData : TeamData
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


teamDataLens : Monocle.Lens.Lens SubState TeamData
teamDataLens =
    Monocle.Lens.Lens .teamData (\td ss -> { ss | teamData = td })


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


type TeamData
    = Unauthenticated
        { apiData : Group.APIData
        }
    | Authenticated
        { apiData : Group.APIData
        , user : Concourse.User
        }


teamData : Group.APIData -> Maybe Concourse.User -> TeamData
teamData apiData user =
    user
        |> Maybe.map (\u -> Authenticated { apiData = apiData, user = u })
        |> Maybe.withDefault (Unauthenticated { apiData = apiData })


apiDataLens : Monocle.Lens.Lens TeamData Group.APIData
apiDataLens =
    Monocle.Lens.Lens apiData setApiData


apiData : TeamData -> Group.APIData
apiData teamData =
    case teamData of
        Unauthenticated { apiData } ->
            apiData

        Authenticated { apiData } ->
            apiData


setApiData : Group.APIData -> TeamData -> TeamData
setApiData apiData teamData =
    case teamData of
        Unauthenticated _ ->
            Unauthenticated { apiData = apiData }

        Authenticated { user } ->
            Authenticated { apiData = apiData, user = user }
