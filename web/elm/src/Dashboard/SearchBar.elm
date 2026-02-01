module Dashboard.SearchBar exposing
    ( handleDelivery
    , searchInputId
    , update
    , view
    )

import Application.Models exposing (Session)
import Dashboard.Filter as Filter
import Dashboard.Models exposing (Model)
import Dict
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import Routes
import Views.SearchBar as SearchBar


searchInputId : String
searchInputId =
    SearchBar.searchInputId


update : Session -> Message -> ET Model
update session msg ( model, effects ) =
    SearchBar.update
        { screenSize = session.screenSize
        , highDensity = model.highDensity
        , clearEffects =
            [ ModifyUrl <|
                Routes.toString <|
                    Routes.Dashboard
                        { searchType = Routes.Normal ""
                        , dashboardView = model.dashboardView
                        }
            ]
        , changeEffects =
            \query ->
                [ ModifyUrl <|
                    Routes.toString <|
                        Routes.Dashboard
                            { searchType = Routes.Normal query
                            , dashboardView = model.dashboardView
                            }
                ]
        }
        msg
        ( model, effects )


handleDelivery : Session -> Delivery -> ET Model
handleDelivery session delivery ( model, effects ) =
    SearchBar.handleDelivery
        { suggestions =
            \m ->
                Filter.suggestions (Filter.filterTeams session m) m.query
        , selectEffects =
            \selectedItem ->
                [ ModifyUrl <|
                    Routes.toString <|
                        Routes.Dashboard
                            { searchType = Routes.Normal selectedItem
                            , dashboardView = model.dashboardView
                            }
                ]
        }
        delivery
        ( model, effects )


view : Session -> Model -> Html Message
view session ({ pipelines } as model) =
    let
        noPipelines =
            pipelines
                |> Maybe.withDefault Dict.empty
                |> Dict.values
                |> List.all List.isEmpty
    in
    SearchBar.view
        { screenSize = session.screenSize
        , highDensity = model.highDensity
        , placeholder = "filter pipelines by name, status, or team"
        , disabled = noPipelines
        , dropdownAbsolute = False
        , suggestions = \m -> Filter.suggestions (Filter.filterTeams session m) m.query
        }
        model
