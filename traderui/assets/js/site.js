$(function () {
  var form = $('#order-ticket');
  $(form).submit(function (event) {
    event.preventDefault();

    var formData = $(form).serialize();
    $.ajax({
      type: "POST",
      url: $(form).attr("action"),
      data: formData
    });
  });
});

setInterval(function () {
  App.orders.fetch({ reset: true });
  App.executions.fetch({ reset: true });
  App.instruments.fetch({ reset: true });
  App.marketdata.fetch({ reset: true });

}, 1000);

var App = new (Backbone.View.extend({
  Models: {},
  Views: {},
  Collections: {},

  events: {
    'click a[data-internal]': function (e) {
      e.preventDefault();
      Backbone.history.navigate(e.target.pathname, { trigger: true });
    }
  },

  start: function (options) {
    console.log(options)
    this.orderTicket = new App.Models.OrderTicket({
      session_ids: options.session_ids,
      symbols: options.symbols,
    });

    this.securityDefinitionForm = new App.Models.SecurityDefinitionForm({
      session_ids: options.session_ids,
      symbols: options.symbols,
    });

    this.marketDataForm = new App.Models.MarketDataForm({
      session_ids: options.session_ids,
      symbols: options.symbols,
    });

    this.orders = new App.Collections.Orders(options.orders);
    this.executions = new App.Collections.Executions(options.executions);
    this.instruments = new App.Collections.Instruments(options.symbols);
    this.marketdata = new App.Collections.MarketData(options.market);
    this.router = new App.Router();
    Backbone.history.start({ pushState: true });
  },

  showOrders: function () {
    var orderTicketView = new App.Views.OrderTicket({ model: this.orderTicket });
    var ordersView = new App.Views.OrdersView({ collection: this.orders });

    $("#app").html(orderTicketView.render().el);
    $("#app").append(ordersView.render().el);
    $("#nav-order").addClass("active");
    $("#nav-execution").removeClass("active");
    $("#nav-secdef").removeClass("active");
    $("#nav-marketdata").removeClass("active");
  },

  showExecutions: function () {
    var orderTicketView = new App.Views.OrderTicket({ model: this.orderTicket });
    var executionsView = new App.Views.Executions({ collection: this.executions });

    $("#app").html(orderTicketView.render().el);
    $("#app").append(executionsView.render().el);
    $("#nav-order").removeClass("active");
    $("#nav-execution").addClass("active");
    $("#nav-secdef").removeClass("active");
    $("#nav-marketdata").removeClass("active");
  },

  showSecurityDefinitions: function () {
    var secDefReq = new App.Views.SecurityDefinitionRequest({ model: this.securityDefinitionForm });
    var secListView = new App.Views.SecurityList({model: this.instruments})
    $("#app").html(secDefReq.render().el);
    $("#app").append(secListView.render().el);
    $("#nav-order").removeClass("active");
    $("#nav-execution").removeClass("active");
    $("#nav-secdef").addClass("active");
    $("#nav-marketdata").removeClass("active");
  },

  showMarketData: function () {
    var mrktDataReq = new App.Views.MarketDataRequest({ model: this.marketDataForm });
    var mrktListView = new App.Views.MarketData({model: this.marketdata})
    $("#app").html(mrktDataReq.render().el);
    $("#app").append(mrktListView.render().el);
    $("#nav-order").removeClass("active");
    $("#nav-execution").removeClass("active");
    $("#nav-secdef").removeClass("active");
    $("#nav-marketdata").addClass("active");
  },

  showOrderDetails: function (id) {
    var order = new App.Models.Order({ id: id });
    order.fetch({
      success: function () {
        console.log(order)
        var orderView = new App.Views.OrderDetails({ model: order });
        $("#app").html(orderView.render().el);
      },
      error: function () {
        console.log('Failed to fetch!');
      }
    });
  },
  showExecutionDetails: function (id) {
    var execution = new App.Models.Execution({ id: id });
    execution.fetch({
      success: function () {
        var executionView = new App.Views.ExecutionDetails({ model: execution });
        $("#app").html(executionView.render().el);
      },
      error: function () {
        console.log('Failed to fetch!');
      }
    });
  }
}))({ el: document.body });

App.Router = Backbone.Router.extend({
  routes: {
    "": "index",
    "orders": "index",
    "executions": "executions",
    "instruments": "instruments",
    "marketdata": "marketdata",
    "secdefs": "secdefs",
    "orders/:id": "orderDetails",
    "executions/:id": "executionDetails",
  },

  index: function () {
    App.showOrders();
  },

  executions: function () {
    App.showExecutions();
  },

  secdefs: function () {
    App.showSecurityDefinitions();
  },

  marketdata: function () {
    App.showMarketData();
  },

  orderDetails: function (id) {
    App.showOrderDetails(id)
  },

  executionDetails: function (id) {
    App.showExecutionDetails(id)
  }
});

App.Models.Order = Backbone.Model.extend({
  urlRoot: "/orders",
});

App.Models.Execution = Backbone.Model.extend({
  urlRoot: "/executions"
});

App.Models.Instruments = Backbone.Model.extend({
  urlRoot: "/instruments"
});

App.Models.MarketData = Backbone.Model.extend({
  urlRoot: "/marketdata"
});

App.Models.SecurityDefinitionRequest = Backbone.Model.extend({
  urlRoot: "securitydefinitionrequest"
});

App.Models.MarketDataRequest = Backbone.Model.extend({
  urlRoot: "marketdatarequest"
});

App.Models.OrderTicket = Backbone.Model.extend({});
App.Models.SecurityDefinitionForm = Backbone.Model.extend({});
App.Models.MarketDataForm = Backbone.Model.extend({});

App.Collections.Orders = Backbone.Collection.extend({
  url: '/orders',
  comparator: 'id'
});

App.Collections.Executions = Backbone.Collection.extend({
  url: '/executions',
  comparator: 'id'
});

App.Collections.Instruments = Backbone.Collection.extend({
  url: '/instruments',
  comparator: 'id'
});

App.Collections.MarketData = Backbone.Collection.extend({
  url: '/marketdata',
  comparator: 'id'
});

App.Views.ExecutionDetails = Backbone.View.extend({
  template: _.template(`
<dl class="dl-horizontal">
  <dt>ID</dt><dd><%= id %></dd> 
	<dt>Symbol</dt><dd><%= symbol %></dd>
	<dt>Quantity</dt><dd><%= quantity %></dd>
	<dt>Session</dt><dd><%= session_id %></dd>
  <dt>Side</dt><dd><%= App.prettySide(side) %></dd>
	<dt>Price</dt><dd><%= price %></dd>
</ul>

</div>
  <a href='#' data-internal='true'>Back</a>
</div>
`),
  render: function () {
    this.$el.html(this.template(this.model.attributes));
    return this;
  },
  events: {
    'click a[data-internal]': function (e) {
      e.preventDefault();
      window.history.back();
    }
  }
});



App.Views.OrderDetails = Backbone.View.extend({
  template: _.template(`
<form class="form-horizontal">
  <div class="form-group">
    <label class="col-sm-2 control-label">ID</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= id %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">ClOrID</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= clord_id %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">OrderID</label>
    <div class="col-sm-10">
    <p class="form-control-static"><%= order_id  %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Symbol</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= symbol %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">PartyID</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= party_id %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Session</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= session_id %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Side</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= App.prettySide(side) %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">OrdType</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= App.prettyOrdType(ord_type) %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Closed</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= closed %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Open</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= open %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Avg Px</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= avg_px %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Security Type</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= security_type %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Security Desc</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= security_desc %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Maturity Month Year</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= maturity_month_year %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Maturity Day</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= maturity_day %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Put or Call</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= put_or_call %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Strike Price</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= strike_price %></p>
    </div>
  </div>

  <% if (open == "0") { %>
  <div class="form-group">
    <label class="col-sm-2 control-label">Quantity</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= quantity %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Price</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= price %></p>
    </div>
  </div>
  <div class="form-group">
    <label class="col-sm-2 control-label">Stop Price</label>
    <div class="col-sm-10">
      <p class="form-control-static"><%= stop_price %></p>
    </div>
  </div>

  <% } else { %>

  <div class="form-group">
    <label for="quantity" class="col-sm-2 control-label">Quantity</label>
    <div class="col-sm-10">
      <input type="number" class="form-control" id="quantity" placeholder="Quantity" value="<%= quantity %>" required>
    </div>
  </div>
  <% } %>

</form>
</div>
  <button class="btn btn-danger cancel" <% if(open == "0"){%>disabled<% }%>>Cancel</button>
  <button class="btn btn-warning amend" <% if(open == "0"){%>disabled<% }%>>Amend</button>
  <button class="btn btn-info back">Back</button>
</div>
`),
  render: function () {
    this.$el.html(this.template(this.model.attributes));
    return this;
  },
  events: {
    'click .back': function (e) {
      window.history.back();
    },

    'click .cancel': function (e) {
      this.model.destroy({
        success: function () {
          Backbone.history.navigate("/orders", { trigger: true });
        },
        error: function (model, response) {
          console.log('Failed to cancel!');
          console.log(model);
          console.log(response);
        }
      });
    },

    'click .amend': function (e) {
      console.log(e)
      var quantity = this.$el.find('#quantity').val();

      this.model.save({ quantity: quantity }, {
        success: function () {
          Backbone.history.navigate("/orders", { trigger: true });
        },
        error: function (model, response) {
          console.log('Failed to amend!');
          console.log(model);
          console.log(response);
        }
      });

    }
  },
});

App.Views.ExecutionRowView = Backbone.View.extend({
  tagName: 'tr',
  template: _.template(`
<td>
<button class="btn btn-info details">Details</button>
</td>
<td><%= symbol %></td>
<td><%= quantity %></td>
<td><%= App.prettySide(side) %></td>
<td><%= price %></td>
<td><%= session_id %></td>
`),


  render: function () {
    this.$el.html(this.template(this.model.attributes));
    return this;
  },
  events: {
    "click .details": "details"
  },
  details: function (e) {
    Backbone.history.navigate("/executions/" + this.model.get("id"), { trigger: true });
  }
});

App.Views.SecurityListRowView = Backbone.View.extend({
  tagName: 'tr',
  template: _.template(`
<td>
<button class="btn btn-info details">Details</button>
</td>
<td><%= request_id %></td>
<td><%= instrument_name %></td>
<td><%= security_desc %></td>
<td><%= security_type %></td>
<td><%= strike_currency %></td>
<td><%= strike_price %></td>
`),


  render: function () {
    this.$el.html(this.template(this.model.attributes));
    return this;
  },
});

App.Views.MarketDataRowView = Backbone.View.extend({
  tagName: 'tr',
  template: _.template(`
<td>
<button class="btn btn-info details">Details</button>
</td>
<td><%= instrumentName %></td>
<td><%= side %></td>
<td><%= contract %></td>
<td><%= price %></td>
<td><%= amount %></td>
<td><%= date %></td>
`),


  render: function () {
    this.$el.html(this.template(this.model.attributes));
    return this;
  },
});


App.Views.OrderRowView = Backbone.View.extend({
  tagName: 'tr',
  template: _.template(`
<td>
<button class="btn btn-danger cancel" <% if(open == "0"){%>disabled<% }%>>Cancel</button>
<button class="btn btn-info details">Details</button>
</td>
<td><%= symbol %></td>
<td><%= quantity %></td>
<td><%= party_id %></td>
<td><%= open %></td>
<td><%= closed %></td>
<td><%= App.prettySide(side) %></td>
<td><%= App.prettyOrdType(ord_type) %></td>
<td><%= price %></td>
<td><%= stop_price %></td>
<td><%= avg_px %></td>
<td><%= session_id %></td>
`),

  render: function () {
    this.$el.html(this.template(this.model.attributes));
    return this;
  },
  events: {
    "click .cancel": "cancel",
    "click .details": "details"
  },
  cancel: function (e) {
    this.model.destroy();
  },

  details: function (e) {
    Backbone.history.navigate("/orders/" + this.model.get("id"), { trigger: true });
  }
});

App.Views.Executions = Backbone.View.extend({
  initialize: function () {
    this.listenTo(this.collection, 'reset', this.addAll);
  },

  render: function () {
    console.log("rendering ex list")
    this.$el.html(`
<table class='table table-striped' id='executions'>
  <thead>
    <tr>
      <th></th>
      <th>Symbol</th>
      <th>Quantity</th>
      <th>Side</th>
      <th>Price</th>
      <th>Session</th>
    </tr>
  </thead>
  <tbody>
  </tbody>
</table>`);

    this.addAll();

    return this;
  },

  addAll: function () {
    this.$("tbody").empty();
    this.collection.forEach(this.addOne, this);
    return this;
  },

  addOne: function (execution) {
    var row = new App.Views.ExecutionRowView({ model: execution });
    this.$("tbody").append(row.render().el);
  }
});

App.Views.SecurityList = Backbone.View.extend({
  initialize: function () {
    console.log("security list view")
    this.listenTo(this.model, 'reset', this.addAll);
  },

  render: function () {
    console.log("renderinggg", this)
    this.$el.html(`
<table class='table table-striped' id='security-list'>
  <thead>
    <tr>
      <th></th>
      <th>RequestID</th>
      <th>Instrument Name</th>
      <th>Sec Desx</th>
      <th>Sec Type</th>
      <th>Strike Currency</th>
      <th>Strike Price</th>
    </tr>
  </thead>
  <tbody>
  </tbody>
</table>`);

    this.addAll();

    return this;
  },

  addAll: function () {
    console.log(this)
    this.$("tbody").empty();
    this.model.models.forEach(this.addOne, this);
    return this;
  },

  addOne: function (instruments) {
    console.log('adding one', instruments)
    var row = new App.Views.SecurityListRowView({ model: instruments });
    this.$("tbody").append(row.render().el);
  }
});

App.Views.MarketData = Backbone.View.extend({
  initialize: function () {
    console.log("market data view")
    this.listenTo(this.model, 'reset', this.addAll);
  },

  render: function () {
    console.log("renderinggg market data", this)
    this.$el.html(`
<table class='table table-striped' id='security-list'>
  <thead>
    <tr>
      <th></th>
      <th>Instrument Name</th>
      <th>Side</th>
      <th>Contracts</th>
      <th>Price</th>
      <th>Amount</th>
      <th>Date</th>
    </tr>
  </thead>
  <tbody>
  </tbody>
</table>`);

    this.addAll();

    return this;
  },

  addAll: function () {
    console.log(this.model)
    this.$("tbody").empty();
    this.model.models.forEach(this.addOne, this);
    return this;
  },

  addOne: function (instruments) {
    console.log('adding one', instruments)
    var row = new App.Views.MarketDataRowView({ model: instruments });
    this.$("tbody").append(row.render().el);
  }
});

App.Views.OrdersView = Backbone.View.extend({
  initialize: function () {
    this.listenTo(this.collection, 'reset', this.addAll);
  },

  render: function () {
    this.$el.html(`
<table class='table table-striped' id='orders'>
  <thead>
    <tr>
      <th></th>
      <th>Symbol</th>
      <th>Quantity</th>
      <th>PartyID</th>
      <th>Open</th>
      <th>Executed</th>
      <th>Side</th>
      <th>Type</th>
      <th>Limit</th>
      <th>Stop</th>
      <th>AvgPx</th>
      <th>Session</th>
    </tr>
  </thead>
  <tbody>
  </tbody>
</table>`);

    this.collection.forEach(this.addOne, this);
    return this;
  },

  addAll: function () {
    this.$("tbody").empty();
    this.collection.forEach(this.addOne, this);
    return this;
  },

  addOne: function (order) {
    var row = new App.Views.OrderRowView({ model: order });
    this.$("tbody").append(row.render().el);
  }
});

App.Views.SecurityDefinitionRequest = Backbone.View.extend({
  template: _.template(`
<form class='form-inline'>
  <p>
    <div class='form-group'>
      <label for="security_request_type">Security Request Type</label>
      <select class='form-control' name='security_request_type'>
        <option value="0">Security Identity and Specifications</option>
        <option value="1">Security Identity for the Specifications Provided</option>
        <option value="2">List Security Types</option>
        <option value="3">List Securities</option>
      </select>
    </div>
  </p>
  <p>
    <div class='form-group'>
      <label for='security_type'>SecurityType</label>
      <select class='form-control' name='security_type' id='security_type' disabled=true>
        <option value='OPT'>Option</option>
      </select>
    </div>

    <div class='form-group'>
      <label for='symbol'>Symbol</label>
      <input type='text' class='form-control' name='symbol' placeholder='symbol'>

    </div>
  </p>
  <p>
  <div class='form-group'>
    <label for="subscription_request_type">Subscription Request Type</label>
    <select class='form-control' name='subscription_request_type'>
      <option value="0">Snapshot</option>
      <option value="1">Snapshot Plus Update</option>
      <option value="2">Disable Update</option>
    </select>
  </div>
</p>
  <p>
  <div class='form-group'>
    <label for='session'>Session</label>
    <select class='form-control' name='session'>
      <% _.each(session_ids, function(i){ %><option><%= i %></option><% }); %>
    </select>
  </div>
  <button type='submit' class='btn btn-default'>Submit</button>
  </p>
</form>
  `),

  events: {
    submit: "submit"
  },

  submit: function (e) {
    e.preventDefault();
    var req = new App.Models.SecurityDefinitionRequest();
    req.set({
      session_id: this.$('select[name=session]').val(),
      security_request_type: this.$('select[name=security_request_type]').val(),
      subscription_request_type: this.$('select[name=subscription_request_type]').val(),
      security_type: this.$('select[name=security_type]').val(),
      symbol: this.$('input[name=symbol]').val(),
    });
    req.save();
  },

  render: function () {
    this.$el.html(this.template(this.model.attributes));
    return this;
  }
});

App.Views.MarketDataRequest = Backbone.View.extend({
  template: _.template(`
<form class='form-inline'>
  <p>
    <div class='form-group'>
      <label for='symbol'>Symbol</label>
      <input type='text' class='form-control' name='symbol' placeholder='symbol'>
    </div>
  </p>
  <p>
  <div class='form-group'>
    <input type="checkbox" id="md_entry_type_1" name="md_entry_type_1" value="Bid">
    <label for="md_entry_type_1"> Bid</label><br>
    <input type="checkbox" id="md_entry_type_2" name="md_entry_type_2" value="Ask">
    <label for="md_entry_type_2"> Ask</label><br>
    <input type="checkbox" id="md_entry_type_3" name="md_entry_type_3" value="Trade">
    <label for="md_entry_type_3"> Trade</label><br>
  </div>
  </p>
  <p>
  <div class='form-group'>
    <label for="subscription_request_type">Subscription Request Type</label>
    <select class='form-control' name='subscription_request_type'>
      <option value="0">Snapshot</option>
      <option value="1">Snapshot Plus Update</option>
      <option value="2">Disable Update</option>
    </select>
  </div>
</p>
  <p>
  <div class='form-group'>
    <label for='session'>Session</label>
    <select class='form-control' name='session'>
      <% _.each(session_ids, function(i){ %><option><%= i %></option><% }); %>
    </select>
  </div>
  <button type='submit' class='btn btn-default'>Submit</button>
  </p>
</form>
  `),

  events: {
    submit: "submit"
  },

  submit: function (e) {
    console.log("submit market data request")
    e.preventDefault();
    var req = new App.Models.MarketDataRequest();
    req.set({
      session_id: this.$('select[name=session]').val(),
      subscription_request_type: this.$('select[name=subscription_request_type]').val(),
      symbol: this.$('input[name=symbol]').val(),
      md_entry_type_1: this.$('input[name=md_entry_type_1]').val(),
      md_entry_type_2: this.$('input[name=md_entry_type_2]').val(),
      md_entry_type_3: this.$('input[name=md_entry_type_3]').val(),
    });
    req.save();
  },

  render: function () {
    this.$el.html(this.template(this.model.attributes));
    return this;
  }
});

App.Views.OrderTicket = Backbone.View.extend({
  template: _.template(`
<form class='form-inline' action='/order' method='POST' id='order-ticket'>
  <p>
    <div class='form-group'>
      <label for='side'>Side</label>
      <select class='form-control' name='side'>
        <option value='1'>Buy</option>
        <option value='2'>Sell</option>
        <option value='5'>Sell Short</option>
        <option value='6'>Sell Short Exempt</option>
        <option value='8'>Cross</option>
        <option value='9'>Cross Short</option>
        <option value='A'>Cross Short Exempt</option>
      </select>
    </div>

    <div class='form-group'>
      <label for='quantity'>Quantity</label>
      <input type='number' class='form-control' name='quantity' placeholder='Quantity' required>
    </div>
  </p>

  <p>
    <div class='form-group'>
      <label for='security_type'>SecurityType</label>
      <select class='form-control' name='security_type' id='security_type' disabled=true>
        <option value='OPT'>Option</option>
      </select>
    </div>

    <div class='form-group'>
      <label for='symbol'>Symbol</label>
      <input type='text' class='form-control' name='symbol' placeholder='symbol'>
    </div>
  </p>
  <p>
    <div class='form-group'>
      <label for='ordType'>Type</label>
      <select class='form-control' name='ordType' id="ordType">
        <option value='1'>Market</option>
        <option value='2'>Limit</option>
        <option value='3'>Stop</option>
        <option value='4'>Stop Limit</option>
      </select>
    </div>

    <div class='form-group'>
      <label for='limit'>Limit</label>
      <input type='number' step='.01' class='form-control' id="limit" placeholder='Limit' name='price' disabled>
    </div>

    <div class='form-group'>
      <label for='stop'>Stop</label>
      <input type='number' step='.01' class='form-control' id="stop" placeholder='Stop' name='stopPrice' disabled>
    </div>
  </p>

  <p>
    <div class='form-group'>
      <label for='party_id'>PartyID</label>
      <input type='text' class='form-control' placeholder='Party ID' name='party_id'>
    </div>

    <div class='form-group'>
      <label for='tif'>TIF</label>
      <select class='form-control' name='tif'>
        <option value='0'>Day</option>
        <option value='3'>IOC</option>
        <option value='2'>OPG</option>
        <option value='1'>GTC</option>
        <option value='5'>GTX</option>
      </select>
    </div>
  </p>

  <p>
    <div class='form-group'>
      <label for='session'>Session</label>
      <select class='form-control' name='session'>
        <% _.each(session_ids, function(i){ %><option><%= i %></option><% }); %>
      </select>
    </div>
  </p>
  <button type='submit' class='btn btn-default'>Submit</button>
</form>
`),
  render: function () {
    this.$el.html(this.template(this.model.attributes));
    return this;
  },

  events: {
    "change #ordType": "updateOrdType",
    "change #security_type": "updateSecurityType",
    submit: "submit"
  },

  submit: function (e) {
    e.preventDefault();
    var order = new App.Models.Order();
    order.set({
      side: this.$('select[name=side]').val(),
      quantity: this.$('input[name=quantity]').val(),
      symbol: this.$('input[name=symbol]').val(),
      ord_type: this.$('select[name=ordType]').val(),
      price: this.$('input[name=price]').val(),
      stop_price: this.$('input[name=stopPrice]').val(),
      party_id: this.$('input[name=party_id]').val(),
      tif: this.$('select[name=tif]').val(),
      session_id: this.$('select[name=session]').val(),
      security_type: this.$('select[name=security_type]').val(),
      put_or_call: this.$('select[name=put_or_call]').val(),
      order_id: this.$('input[name=order_id]').val(),
    });

    order.save();
  },

  updateSecurityType: function () {
    switch (this.$("#security_type option:selected").text()) {
      case "Common Stock":
        this.$("#maturity_month_year").attr({ disabled: true, required: false });
        this.$("#maturity_day").attr({ disabled: true });
        this.$("#put_or_call").attr({ disabled: true, required: false });
        this.$("#strike_price").attr({ disabled: true, required: false });
        break;
      case "Future":
        this.$("#maturity_month_year").attr({ disabled: false, required: true });
        this.$("#maturity_day").attr({ disabled: false });
        this.$("#put_or_call").attr({ disabled: true, required: false });
        this.$("#strike_price").attr({ disabled: true, required: false });
        break;
      case "Option":
        this.$("#maturity_month_year").attr({ disabled: false, required: true });
        this.$("#maturity_day").attr({ disabled: false });
        this.$("#put_or_call").attr({ disabled: false, required: true });
        this.$("#strike_price").attr({ disabled: false, required: true });
        break;
    }
  },

  updateOrdType: function () {
    switch (this.$("#ordType option:selected").text()) {
      case "Limit":
        this.$("#limit").prop("disabled", false);
        this.$("#limit").prop("required", true);
        this.$("#stop").prop("disabled", true);
        this.$("#stop").prop("required", false);
        break;

      case "Stop":
        this.$("#limit").prop("disabled", true);
        this.$("#limit").prop("required", false);
        this.$("#stop").prop("disabled", false);
        this.$("#stop").prop("required", true);
        break;

    }
  },

  updateOrdType: function () {
    switch (this.$("#ordType option:selected").text()) {
      case "Limit":
        this.$("#limit").prop("disabled", false);
        this.$("#limit").prop("required", true);
        this.$("#stop").prop("disabled", true);
        this.$("#stop").prop("required", false);
        break;

      case "Stop":
        this.$("#limit").prop("disabled", true);
        this.$("#limit").prop("required", false);
        this.$("#stop").prop("disabled", false);
        this.$("#stop").prop("required", true);
        break;

      case "Stop Limit":
        this.$("#limit").prop("disabled", false);
        this.$("#limit").prop("required", true);
        this.$("#stop").prop("disabled", false);
        this.$("#stop").prop("required", true);
        break;

      default:
        this.$("#limit").prop("disabled", true);
        this.$("#stop").prop("disabled", true);
        this.$("#limit").prop("required", false);
        this.$("#stop").prop("required", false);
    }
  }
});

App.prettySide = function (sideEnum) {
  switch (sideEnum) {
    case "1":
      return "Buy";
    case "2":
      return "Sell";
    case "5":
      return "Sell Short";
    case "6":
      return "Sell Short Exempt";
    case "8":
      return "Cross";
    case "9":
      return "Cross Short";
    case "A":
      return "Cross Short Exempt";
  }

  return sideEnum;
};

App.prettyOrdType = function (ordTypeEnum) {
  switch (ordTypeEnum) {
    case "1": return "Market";
    case "2": return "Limit";
    case "3": return "Stop";
    case "4": return "Stop Limit";
  };

  return ordTypeEnum;
};




