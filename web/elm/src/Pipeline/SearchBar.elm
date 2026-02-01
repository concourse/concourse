module Pipeline.SearchBar exposing
    ( handleDelivery
    , searchInputId
    , update
    , view
    )

import EffectTransformer exposing (ET)
import Html exposing (Html)
import Message.Message exposing (Message)
import Message.Subscription exposing (Delivery(..))
import Pipeline.Filter as Filter
import ScreenSize exposing (ScreenSize)
import Views.SearchBar as SearchBar


searchInputId : String
searchInputId =
    SearchBar.searchInputId


update : Message -> ET { a | query : String, dropdown : SearchBar.Dropdown, screenSize : ScreenSize }
update msg ( model, effects ) =
    SearchBar.update
        { screenSize = model.screenSize
        , highDensity = False
        , clearEffects = []
        , changeEffects = \_ -> []
        }
        msg
        ( model, effects )


handleDelivery : Delivery -> ET { a | query : String, dropdown : SearchBar.Dropdown, screenSize : ScreenSize }
handleDelivery delivery ( model, effects ) =
    let
        updatedModel =
            case delivery of
                WindowResized width _ ->
                    { model | screenSize = ScreenSize.fromWindowSize width }

                _ ->
                    model
    in
    SearchBar.handleDelivery
        { suggestions = \m -> Filter.suggestions m.query
        , selectEffects = \_ -> []
        }
        delivery
        ( updatedModel, effects )


view : { a | screenSize : ScreenSize } -> { b | query : String, dropdown : SearchBar.Dropdown } -> Html Message
view session model =
    SearchBar.view
        { screenSize = session.screenSize
        , highDensity = False
        , placeholder = "filter jobs by name or status"
        , disabled = False
        , dropdownAbsolute = True
        , suggestions = \m -> Filter.suggestions m.query
        }
        model
